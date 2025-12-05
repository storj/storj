// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package lrucache

import (
	"container/list"
	"sync"
)

// HandlePool is an LRU cache of elements associated with opening and closing
// functions. For example, files which must be opened before use and must be
// closed when expiring from the cache.
//
// Importantly, "open" operations are expected to be fairly slow, and are
// allowed to happen concurrently with other "open" operations and with ongoing
// use of the HandlePool.
type HandlePool[K comparable, V any] struct {
	capacity int
	cache    map[K]*list.Element
	queue    *list.List
	lock     sync.Mutex

	// Opener is called when a key is not in the cache and must be opened.
	// Neither the HandlePool lock nor the handleElement's lock is held while
	// Opener is called.
	//
	// Opener has no built-in facility for error handling; the caller must
	// store errors if necessary in V or in an external variable.
	Opener func(key K) V

	// Closer is called when a key is removed from the cache. The HandlePool
	// lock is not held when Closer is called, but the handleElement's lock
	// is held. Closer is responsible for cleaning up any resources associated
	// with V.
	//
	// Closer has no built-in facility for error handling; the caller must
	// handle or store errors on its own if necessary. The key and value will
	// be removed from the HandlePool regardless.
	Closer func(key K, value V)
}

type handleElement[K comparable, V any] struct {
	key         K
	value       V
	initialized bool
	mu          sync.Mutex
	// If initialized is false, this cond may be waited on until it
	// is true. It is associated with mu.
	ready sync.Cond
	// handleElements are reference-counted. This is in order to handle
	// cases like "handle gets evicted before it finisihed opening" or
	// "handle is evicted while it is still in use by a caller."
	refCount int
}

// NewHandlePool creates a new HandlePool with the given capacity.
func NewHandlePool[K comparable, V any](capacity int, opener func(key K) V, closer func(key K, value V)) *HandlePool[K, V] {
	return &HandlePool[K, V]{
		capacity: capacity,
		cache:    make(map[K]*list.Element),
		queue:    list.New(),
		Opener:   opener,
		Closer:   closer,
	}
}

// Get returns the value associated with the key, opening it if necessary.
// Get will block if Opener blocks, whether because this call is opening
// the value another call is opening the value and hasn't finished.
//
// The caller must call the release function when done with the value.
//
// If a new value is opened and this would exceed the capacity of the pool,
// the oldest value is removed before proceeding.
func (pool *HandlePool[K, V]) Get(key K) (value V, release func()) {
	pool.lock.Lock()

	if elem, ok := pool.cache[key]; ok {
		pool.queue.MoveToFront(elem)
		handle := elem.Value.(*handleElement[K, V])

		handle.mu.Lock()
		handle.refCount++
		needInitialization := !handle.initialized
		handle.mu.Unlock()

		pool.lock.Unlock()

		if needInitialization {
			handle.mu.Lock()
			for !handle.initialized {
				handle.ready.Wait()
			}
			handle.mu.Unlock()
		}
		return handle.value, func() { pool.decref(handle) }
	}

	// we don't call the opener yet, so that it can be done while not holding pool.lock
	handle := &handleElement[K, V]{key: key}
	handle.ready.L = &handle.mu
	elem := pool.queue.PushFront(handle)
	pool.cache[key] = elem

	// we also don't close expired handles yet, so that they can be closed while not
	// holding pool.lock
	var (
		toBeClosed *handleElement[K, V]
		doClose    bool
	)
	if pool.queue.Len() > pool.capacity {
		lastElem := pool.queue.Back()
		toBeClosed = pool.queue.Remove(lastElem).(*handleElement[K, V])
		doClose = true
		delete(pool.cache, toBeClosed.key)
	}

	// a handle starts life with 2 refCount (one for the caller to take
	// on, and one for the pool). it is closed at 0 refCount.
	handle.refCount = 2

	pool.lock.Unlock()

	// After this point, many goroutines might have references to handle.
	// They are expected to wait until initialized is true.

	if doClose {
		pool.decref(toBeClosed)
	}

	newValue := pool.Opener(key)

	handle.mu.Lock()
	handle.value = newValue
	handle.initialized = true
	handle.ready.Broadcast()
	handle.mu.Unlock()

	return handle.value, func() { pool.decref(handle) }
}

func (pool *HandlePool[K, V]) decref(handleElement *handleElement[K, V]) {
	handleElement.mu.Lock()
	defer handleElement.mu.Unlock()
	for !handleElement.initialized {
		handleElement.ready.Wait()
	}
	handleElement.refCount--
	if handleElement.refCount == 0 {
		pool.Closer(handleElement.key, handleElement.value)
	}
}

// Peek gets the value associated with the key if it is in the cache, without
// opening it. If the key is not in the cache, ok will be false and release
// will be nil.
//
// If ok is true, the caller must call the release function when done with the
// value.
//
// Note that if the key is currently being opened, Peek will return false,
// as though the value were not in the pool.
func (pool *HandlePool[K, V]) Peek(key K) (value V, release func(), ok bool) {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	elem, ok := pool.cache[key]
	if !ok {
		return value, nil, false
	}

	handle := elem.Value.(*handleElement[K, V])
	handle.mu.Lock()
	defer handle.mu.Unlock()

	if !handle.initialized {
		return value, nil, false
	}
	handle.refCount++

	return handle.value, func() { pool.decref(handle) }, true
}

// Delete removes a handle from the pool, closing it if not in use. If the
// handle is in use, it will be closed when released. If the handle is not in
// the pool, Delete has no effect.
//
// The value associated with the key is returned if it was in the pool, along
// with ok=true. If ok=false, the key was not in the pool and value is the
// zero value.
func (pool *HandlePool[K, V]) Delete(key K) (value V, ok bool) {
	pool.lock.Lock()

	var toBeClosed *handleElement[K, V]
	if elem, ok := pool.cache[key]; ok {
		toBeClosed = elem.Value.(*handleElement[K, V])
		delete(pool.cache, key)
		pool.queue.Remove(elem)
	}

	pool.lock.Unlock()

	if toBeClosed != nil {
		pool.decref(toBeClosed)
		return toBeClosed.value, true
	}
	return value, false
}

// CloseAll removes all handles from the pool, closing those that are no longer
// in use. Importantly, this does not necessarily close all handles, as they may
// still be in use by callers. Such handles will be closed when released.
func (pool *HandlePool[K, V]) CloseAll() {
	pool.lock.Lock()

	toBeClosed := make([]*handleElement[K, V], 0, len(pool.cache))
	for _, elem := range pool.cache {
		handle := elem.Value.(*handleElement[K, V])
		toBeClosed = append(toBeClosed, handle)
	}
	pool.cache = make(map[K]*list.Element)
	pool.queue.Init()

	pool.lock.Unlock()

	for _, handle := range toBeClosed {
		pool.decref(handle)
	}
}

// ForEach calls the given function for each key-value pair in a snapshot
// of the cache. If a handle is still being initialized, it will not be
// included. Each handle's lock is held in turn while fn is called.
func (pool *HandlePool[K, V]) ForEach(fn func(key K, value V)) {
	var handles []*handleElement[K, V]
	func() {
		pool.lock.Lock()
		defer pool.lock.Unlock()
		handles = make([]*handleElement[K, V], 0, len(pool.cache))
		for _, elem := range pool.cache {
			handle := elem.Value.(*handleElement[K, V])
			handle.mu.Lock()
			handle.refCount++
			handle.mu.Unlock()
			handles = append(handles, handle)
		}
	}()

	for _, handle := range handles {
		func() {
			handle.mu.Lock()
			defer handle.mu.Unlock()
			if handle.initialized {
				fn(handle.key, handle.value)
			}
		}()
		pool.decref(handle)
	}
}
