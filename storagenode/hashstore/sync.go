// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"sync"

	"storj.io/drpc/drpcsignal"
)

//
// context/signal aware mutex
//

type mutex struct {
	ch chan struct{}
}

func newMutex() *mutex {
	return &mutex{ch: make(chan struct{}, 1)}
}

func (s *mutex) WaitLock() { s.ch <- struct{}{} }

func (s *mutex) Lock(ctx context.Context, closed *drpcsignal.Signal) error {
	if err := ctx.Err(); err != nil {
		return err
	} else if err := signalError(closed); err != nil {
		return err
	}
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-closed.Signal():
		return signalError(closed)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *mutex) Unlock() { <-s.ch }

//
// context/signal aware rw-mutex
//

type rwMutexSlot struct {
	ch   chan struct{}
	read bool // is a pending read lock
	next *rwMutexSlot
	prev *rwMutexSlot
}

func (rw *rwMutexSlot) remove() {
	prev, next := rw.prev, rw.next
	if next != nil {
		next.prev = prev
	}
	if prev != nil {
		prev.next = next
	}
	rw.prev, rw.next = nil, nil
}

var rwMutexPool = sync.Pool{
	New: func() any { return &rwMutexSlot{ch: make(chan struct{}, 1)} },
}

// rwMutex is a context-aware fair read/write mutex. The zero value is valid.
type rwMutex struct {
	mu     sync.Mutex
	head   *rwMutexSlot
	tail   *rwMutexSlot
	reads  int
	writes bool
}

func newRWMutex() *rwMutex { return &rwMutex{} }

func (rwm *rwMutex) Unlock()   { rwm.unlock(false) }
func (rwm *rwMutex) WaitLock() { _ = rwm.Lock(context.Background(), new(drpcsignal.Signal)) }
func (rwm *rwMutex) Lock(ctx context.Context, sig *drpcsignal.Signal) error {
	return rwm.lock(ctx, sig, false)
}

func (rwm *rwMutex) RUnlock() { rwm.unlock(true) }
func (rwm *rwMutex) RLock(ctx context.Context, sig *drpcsignal.Signal) error {
	return rwm.lock(ctx, sig, true)
}

func (rwm *rwMutex) lock(ctx context.Context, sig *drpcsignal.Signal, read bool) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	} else if err := signalError(sig); err != nil {
		return err
	}

	slot, _ := rwMutexPool.Get().(*rwMutexSlot)
	defer rwMutexPool.Put(slot)

	rwm.mu.Lock()
	slot.read = read

	if rwm.head == nil { // the list is empty, set head and tail
		rwm.head = slot
		rwm.tail = slot
	} else { // the list is not empty, append it to tail
		slot.prev = rwm.tail
		rwm.tail.next = slot
		rwm.tail = slot
	}

	rwm.processLocked()
	rwm.mu.Unlock()

	select {
	case <-slot.ch:
	case <-ctx.Done():
		err = ctx.Err()
	case <-sig.Signal():
		err = signalError(sig)
	}

	rwm.mu.Lock()
	if rwm.head == slot {
		rwm.head = slot.next
	}
	if rwm.tail == slot {
		rwm.tail = slot.prev
	}
	slot.remove()

	// if we got an error but processLocked also signaled us that we acquired, then immediately drop
	// to return the error.
	if err != nil && len(slot.ch) == 1 {
		<-slot.ch
		rwm.unlockLocked(read)
	}
	rwm.mu.Unlock()

	return err
}

func (rwm *rwMutex) unlock(read bool) {
	rwm.mu.Lock()
	rwm.unlockLocked(read)
	rwm.mu.Unlock()
}

func (rwm *rwMutex) unlockLocked(read bool) {
	if read {
		rwm.reads--
	} else {
		rwm.writes = false
	}
	rwm.processLocked()
}

func (rwm *rwMutex) processLocked() {
	for rwm.head != nil {
		slot := rwm.head

		// if we're trying to write and there are active reads or if we already have a write mutex
		// then we're done.
		if rwm.writes || (!slot.read && rwm.reads != 0) {
			return
		}

		// the mutex is available so signal the next waiter and update the state.
		slot.ch <- struct{}{}
		if slot.read {
			rwm.reads++
		} else {
			rwm.writes = true
		}

		// if we cleared out the head, then we also need to clear out the tail.
		rwm.head = slot.next
		if rwm.head == nil {
			rwm.tail = nil
		}
	}
}
