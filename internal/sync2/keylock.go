// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"sync"
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
	locks sync.Map
}

type UnlockFunc func()

// NewKeyLock create a new KeyLock
func NewKeyLock() *KeyLock {
	return &KeyLock{}
}

// Lock the provided key
func (l *KeyLock) Lock(key interface{}) UnlockFunc {
	lock := l.getLock(key)
	lock.Lock()
	return lock.Unlock
}

// RLock the provided key
func (l *KeyLock) RLock(key interface{}) UnlockFunc {
	lock := l.getLock(key)
	lock.RLock()
	return lock.RUnlock
}

// getLock will atomically load the RWMutex for this key. If one does not yet
// exist, it will be lazily, atomically created. The resulting RWMutex is
// returned.
func (l *KeyLock) getLock(key interface{}) *sync.RWMutex {
	res, _ := l.locks.LoadOrStore(key, &sync.RWMutex{})
	return res.(*sync.RWMutex)
}
