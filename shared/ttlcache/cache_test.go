// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package ttlcache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestCache returns a cache driven by a clock the test advances by hand.
func newTestCache[T any](ttl time.Duration, compute func() T) (_ *Cache[T], advance func(time.Duration)) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := New(ttl, compute)
	cache.clock = func() time.Time { return now }
	return cache, func(d time.Duration) { now = now.Add(d) }
}

func TestCache(t *testing.T) {
	t.Parallel()

	t.Run("reuses value until it expires", func(t *testing.T) {
		calls := 0
		cache, advance := newTestCache(time.Minute, func() int { calls++; return calls })

		require.Equal(t, 1, cache.Get())
		advance(59 * time.Second)
		require.Equal(t, 1, cache.Get())
		advance(time.Second)
		require.Equal(t, 2, cache.Get())
		require.Equal(t, 2, calls)
	})

	t.Run("expiry is measured from the end of the compute", func(t *testing.T) {
		var cache *Cache[int]
		var advance func(time.Duration)
		calls := 0
		cache, advance = newTestCache(time.Minute, func() int {
			calls++
			advance(5 * time.Minute) // a compute slower than the ttl
			return calls
		})

		require.Equal(t, 1, cache.Get())
		require.Equal(t, 1, cache.Get(), "the entry should still be usable")
		require.Equal(t, 1, calls)
	})

	t.Run("invalidate forces a recompute", func(t *testing.T) {
		calls := 0
		cache, _ := newTestCache(time.Minute, func() int { calls++; return calls })

		require.Equal(t, 1, cache.Get())
		cache.Invalidate()
		require.Equal(t, 2, cache.Get())
	})

	t.Run("non-positive ttl disables caching", func(t *testing.T) {
		calls := 0
		cache, _ := newTestCache(0, func() int { calls++; return calls })

		require.Equal(t, 1, cache.Get())
		require.Equal(t, 2, cache.Get())
	})
}

func TestCacheConcurrent(t *testing.T) {
	t.Parallel()

	const concurrency = 32

	var calls atomic.Int64
	cache, _ := newTestCache(time.Minute, func() int64 { return calls.Add(1) })

	var group sync.WaitGroup
	for range concurrency {
		group.Go(func() {
			require.EqualValues(t, 1, cache.Get())
		})
	}
	group.Wait()

	require.EqualValues(t, 1, calls.Load(), "a burst of callers should trigger a single compute")
}
