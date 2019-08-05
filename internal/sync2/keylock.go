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

// NewKeyLock create a new KeyLock
func NewKeyLock() *KeyLock {
	return &KeyLock{}
}

// Lock the provided key
func (l *KeyLock) Lock(key interface{}) {
	lock := l.getNewLock(key)
	lock.Lock()
}

// Unlock the provided key
func (l *KeyLock) Unlock(key interface{}) {
	lock := l.getExistingLock(key)
	lock.Unlock()
}

// RLock the provided key
func (l *KeyLock) RLock(key interface{}) {
	lock := l.getNewLock(key)
	lock.RLock()
}

// RUnlock the provided key
func (l *KeyLock) RUnlock(key interface{}) {
	lock := l.getExistingLock(key)
	lock.RUnlock()
}

// getNewLock will atomically load the RWMutex for this key. If one does not yet
// exist, it will be lazily, atomically created. The resulting RWMutex is
// returned.
func (l *KeyLock) getNewLock(key interface{}) *sync.RWMutex {
	res, _ := l.locks.LoadOrStore(key, &sync.RWMutex{})
	return res.(*sync.RWMutex)
}

// getExistingLock is a more efficient way to load a RWMutex for a key which you
// know already exists (such as when calling unlock).
func (l *KeyLock) getExistingLock(key interface{}) *sync.RWMutex {
	res, _ := l.locks.Load(key)
	return res.(*sync.RWMutex)
}
