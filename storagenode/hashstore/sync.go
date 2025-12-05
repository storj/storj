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
	case <-signalChan(closed):
		return signalError(closed)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *mutex) Unlock() { <-s.ch }

//
// context/signal aware rw-mutex
//

// rwMutexWaiter repesents a call to Lock or RLock waiting to acquire the lock. It is acquired when
// the channel is sent on. It maintains a doubly linked list of other waiters to allow for efficient
// removal.
type rwMutexWaiter struct {
	ch    chan struct{}
	read  bool // is a pending read lock
	newer *rwMutexWaiter
	older *rwMutexWaiter
}

// rwMutexWaiterList is a doubly linked list of rwMutexWaiters. It allows pushing a new waiter on to
// the end of the list (making it the newest), and efficient removal of any waiter from the list.
type rwMutexWaiterList struct {
	oldest *rwMutexWaiter
	newest *rwMutexWaiter
}

// pushWaiter pushes a waiter to the end of the list, making it the newest (and also potentially the
// oldest if the list was empty).
func (list *rwMutexWaiterList) pushWaiter(waiter *rwMutexWaiter) {
	if list.oldest == nil {
		list.oldest = waiter
		list.newest = waiter
	} else {
		waiter.older = list.newest
		list.newest.newer = waiter
		list.newest = waiter
	}
}

// removeWaiter removes a waiter from the list. It is idempotent in that removing a waiter that was
// already removed does nothing. It clears out the older and newer pointers of the waiter.
func (list *rwMutexWaiterList) removeWaiter(waiter *rwMutexWaiter) {
	if waiter.older != nil {
		waiter.older.newer = waiter.newer
	}
	if waiter.newer != nil {
		waiter.newer.older = waiter.older
	}
	if list.oldest == waiter {
		list.oldest = waiter.newer
	}
	if list.newest == waiter {
		list.newest = waiter.older
	}
	waiter.older, waiter.newer = nil, nil
}

// rwMutexWaiterPool is a sync.Pool of rwMutexWaiters to avoid allocations of the waiters in the
// common case. The waiters do not outlive the stack frame of the lock function, but the compiler
// isn't smart enough to be able to stack allocate them. This is the next best thing.
var rwMutexWaiterPool = sync.Pool{
	New: func() any { return &rwMutexWaiter{ch: make(chan struct{}, 1)} },
}

// rwMutex is a context-aware fair read/write mutex. The zero value is valid and represents an
// unlimited active read limit.
type rwMutex struct {
	mu              sync.Mutex
	waiters         rwMutexWaiterList
	activeReads     int
	activeReadLimit int
	pendingWrites   int
	writeHeld       bool
	syncLifo        bool
}

// newRWMutex allocates an rwMutex with the given active read limit and sync lifo setting.
// If the active read limit is zero, then there is no limit to the number of active reads.
func newRWMutex(activeReadLimit int, syncLifo bool) *rwMutex {
	return &rwMutex{activeReadLimit: activeReadLimit, syncLifo: syncLifo}
}

// Unlock unlocks the rwMutex.
func (rwm *rwMutex) Unlock() { rwm.unlock(false) }

// WaitLock is like Lock but cannot be cancelled and so does not return an error.
func (rwm *rwMutex) WaitLock() { _ = rwm.Lock(context.Background(), nil) }

// Lock locks the rwMutex.
func (rwm *rwMutex) Lock(ctx context.Context, closed *drpcsignal.Signal) error {
	return rwm.lock(ctx, closed, false)
}

// RUnlock unlocks the rwMutex for reading.
func (rwm *rwMutex) RUnlock() { rwm.unlock(true) }

// RLock locks the rwMutex for reading.
func (rwm *rwMutex) RLock(ctx context.Context, closed *drpcsignal.Signal) error {
	return rwm.lock(ctx, closed, true)
}

// lock blocks until the requested kind of mutex is acquired, returning an error if it was not
// acquired due to the context being canceled or the signal being set.
func (rwm *rwMutex) lock(ctx context.Context, closed *drpcsignal.Signal, read bool) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	} else if err := signalError(closed); err != nil {
		return err
	}

	// first do a fast path check to see if the mutex is unlocked which is the common case. if it is
	// we can just barge in and take it immediately without doing any expensive waiter management.
	rwm.mu.Lock()
	if rwm.activeReads == 0 && !rwm.writeHeld {
		if read {
			rwm.activeReads++
		} else {
			rwm.writeHeld = true
		}
		rwm.mu.Unlock()
		return nil
	}

	// acquire a waiter from the pool and set its read flag to the correct value.
	waiter, _ := rwMutexWaiterPool.Get().(*rwMutexWaiter)
	defer rwMutexWaiterPool.Put(waiter)
	waiter.read = read

	// while the lock is held, update our state, push the waiter to the newest slot in the list and
	// process any mutexes that can be acquired.
	if !read {
		rwm.pendingWrites++
	}
	rwm.waiters.pushWaiter(waiter)
	rwm.processLocked()
	rwm.mu.Unlock()

	// wait for either the waiter to be told that the lock is acquired, the context to be canceled,
	// or the signal to be set.
	select {
	case <-waiter.ch:
	case <-ctx.Done():
		err = ctx.Err()
	case <-signalChan(closed):
		err = signalError(closed)
	}

	// remove the waiter from the list and update our state.
	rwm.mu.Lock()
	rwm.waiters.removeWaiter(waiter)
	if !read {
		rwm.pendingWrites--
	}

	// if we got an error but processLocked also told us that we acquired, then we should prefer the
	// error condition so we have to clear out the channel and return the error. it is safe to look
	// at the length of the channel because only processLocked sends on any of the waiter channels
	// and it only does that while holding rwm.mu, and we're holding that mutex right now and the
	// waiter has been removed from the list that processLocked looks at.
	if err != nil && len(waiter.ch) == 1 {
		<-waiter.ch
		rwm.unlockLocked(read)
	}
	rwm.mu.Unlock()

	return err
}

// unlock unlocks the requested mutex.
func (rwm *rwMutex) unlock(read bool) {
	rwm.mu.Lock()
	rwm.unlockLocked(read)
	rwm.mu.Unlock()
}

// unlockLocked is a helper function that does the unlock work that, like all functions named with
// the suffix Locked, must only be called while holding rwm.mu.
func (rwm *rwMutex) unlockLocked(read bool) {
	if read {
		rwm.activeReads--
	} else {
		rwm.writeHeld = false
	}
	rwm.processLocked()
}

// processLocked is the function called during events that might change which mutexes can be
// acquired: adding a new waiter or on unlock. it is responsible to maintain the invariant that
// write locks are exclusive to all other locks and read locks are exclusive only to write locks
// while signaling which waiters now have the lock. it can operate in either FIFO or LIFO mode which
// is mostly the same except that LIFO prefers the newest added waiters over the oldest and some
// special handling for when there is a pending writer.
func (rwm *rwMutex) processLocked() {
	slot := &rwm.waiters.oldest
	if rwm.syncLifo {
		slot = &rwm.waiters.newest
	}

	// we allow a batch of reads to proceed even if we have a write pending if we're in lifo mode
	// and there are no active reads or writes. this is to prevent the situation where we have a
	// very old and stubborn pending write that would then only allow a single read to proceed at
	// a time, hurting read throughput.
	batchReads := rwm.activeReads == 0 && !rwm.writeHeld

	for *slot != nil {
		waiter := *slot

		// global properties that don't depend on the kind of lock
		if rwm.writeHeld {
			return
		} else if rwm.activeReadLimit > 0 && rwm.activeReads >= rwm.activeReadLimit {
			return
		}

		// write locks are exclusive with active reads
		if !waiter.read && rwm.activeReads > 0 {
			return
		}

		// if we have pending writes, we can't allow new reads unless we have no readers.
		// we only need to do this check if we are in LIFO mode and we aren't batching reads.
		if rwm.syncLifo && !batchReads && waiter.read && rwm.pendingWrites > 0 && rwm.activeReads > 0 {
			return
		}

		waiter.ch <- struct{}{}

		if waiter.read {
			rwm.activeReads++
		} else {
			rwm.writeHeld = true
		}

		rwm.waiters.removeWaiter(waiter) // N.B. this is idempotent
	}
}
