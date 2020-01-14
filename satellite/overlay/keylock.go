// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package overlay

import (
	"sync"

	"storj.io/common/storj"
)

// KeyLock provides per-key RW locking. Locking is key-specific, meaning lock
// contention only exists for locking calls using the same key parameter. As
// with all locks, do not call Unlock() or RUnlock() before a corresponding
// Lock() or RLock() call.
//
// Note on memory usage: internally KeyLock lazily and atomically creates a
// separate sync.RWMutex for each key. To maintain synchronization guarantees,
// these interal mutexes are not freed until the entire KeyLock instance is
// freed.
type KeyLock struct {
	locksMu sync.Mutex
	locks   map[storj.NodeID]*sync.RWMutex
}

// UnlockFunc is the function to unlock the associated successful lock
type UnlockFunc func()

// NewKeyLock create a new KeyLock
func NewKeyLock() *KeyLock {
	return &KeyLock{
		locks: make(map[storj.NodeID]*sync.RWMutex),
	}
}

// Lock the provided key. Returns the unlock function.
func (l *KeyLock) Lock(nodeID storj.NodeID) UnlockFunc {
	lock := l.getLock(nodeID)
	lock.Lock()
	return lock.Unlock
}

// RLock the provided key. Returns the unlock function.
func (l *KeyLock) RLock(nodeID storj.NodeID) UnlockFunc {
	lock := l.getLock(nodeID)
	lock.RLock()
	return lock.RUnlock
}

// getLock will atomically load the RWMutex for this key. If one does not yet
// exist, it will be lazily and atomically created. The resulting RWMutex is
// returned.
func (l *KeyLock) getLock(nodeID storj.NodeID) *sync.RWMutex {
	l.locksMu.Lock()
	defer l.locksMu.Unlock()

	lo, ok := l.locks[nodeID]
	if ok {
		return lo
	}
	mu := &sync.RWMutex{}
	l.locks[nodeID] = mu
	return mu
}
