// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package ttlcache provides a single-value cache that recomputes its contents
// after a time to live elapses.
package ttlcache

import (
	"sync"
	"sync/atomic"
	"time"
)

// Cache holds a single value computed on demand and reused until its time to
// live elapses. It is intended for values that are expensive to compute and
// cheap to keep slightly out of date. A non-positive ttl disables caching: every
// Get recomputes.
//
// The zero value is not usable; construct one with New.
type Cache[T any] struct {
	ttl     time.Duration
	compute func() T
	clock   func() time.Time

	// mu serializes refreshes so a burst of concurrent Get calls triggers at most
	// one compute. It is deliberately never taken by Invalidate; see there.
	mu      sync.Mutex
	current atomic.Pointer[entry[T]]
}

// entry is a cached value together with the time it goes stale.
type entry[T any] struct {
	value  T
	expiry time.Time
}

// New returns a Cache that reuses the result of compute for ttl. A non-positive
// ttl disables caching.
func New[T any](ttl time.Duration, compute func() T) *Cache[T] {
	return &Cache[T]{
		ttl:     ttl,
		compute: compute,
		clock:   time.Now,
	}
}

// Get returns the cached value, or computes and caches a new one if there is
// none or it has expired. Concurrent callers that arrive while a compute is in
// flight wait for it and share its result rather than each computing in turn.
//
// The time to live is measured from the moment compute returns, so a compute
// slower than the ttl still yields a usable cache entry.
func (c *Cache[T]) Get() T {
	if c.ttl <= 0 {
		return c.compute()
	}

	// Sampled once so that callers queued behind a refresh take its result
	// instead of immediately recomputing in turn.
	now := c.clock()
	if cached := c.current.Load(); cached != nil && now.Before(cached.expiry) {
		return cached.value
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if cached := c.current.Load(); cached != nil && now.Before(cached.expiry) {
		return cached.value
	}

	value := c.compute()
	c.current.Store(&entry[T]{
		value:  value,
		expiry: c.clock().Add(c.ttl),
	})
	return value
}

// Invalidate drops any cached value, forcing the next Get to recompute.
//
// It never blocks and never takes the cache's own lock. That is a guarantee, not
// an implementation detail: callers invalidate while holding locks that compute
// itself takes, so a lock-guarded Invalidate would invert that order and
// deadlock.
func (c *Cache[T]) Invalidate() {
	c.current.Store(nil)
}
