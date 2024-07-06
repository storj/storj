// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package lrucache

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

type handlePoolCounter struct {
	t          *testing.T
	pool       *HandlePool[int, string]
	mu         sync.Mutex // only must be held while accessing the following maps
	isOpen     map[int]bool
	openCount  map[int]int
	closeCount map[int]int
}

func (hpc *handlePoolCounter) Get(key int) string {
	val, release := hpc.pool.Get(key)
	defer release()
	return val
}

func (hpc *handlePoolCounter) Open(key int) string {
	hpc.mu.Lock()
	defer hpc.mu.Unlock()

	require.False(hpc.t, hpc.isOpen[key], "handle already open")
	hpc.isOpen[key] = true
	hpc.openCount[key]++

	return strconv.Itoa(key)
}

func (hpc *handlePoolCounter) Close(key int, value string) {
	hpc.mu.Lock()
	defer hpc.mu.Unlock()

	require.True(hpc.t, hpc.isOpen[key], "handle not open")
	hpc.isOpen[key] = false
	hpc.closeCount[key]++

	s := strconv.Itoa(key)
	require.Equal(hpc.t, s, value)
}

func newHandlePoolCounter(t *testing.T, capacity int) *handlePoolCounter {
	pool := NewHandlePool[int, string](capacity, nil, nil)
	hpc := &handlePoolCounter{
		t:          t,
		pool:       pool,
		isOpen:     make(map[int]bool),
		openCount:  make(map[int]int),
		closeCount: make(map[int]int),
	}
	pool.Opener = hpc.Open
	pool.Closer = hpc.Close

	return hpc
}

func TestHandlePool_Get(t *testing.T) {
	// Test that HandlePool even resembles something that works.
	hp := newHandlePoolCounter(t, 3)
	require.Equal(t, "1", hp.Get(1))
	require.Equal(t, "2", hp.Get(2))
	require.Equal(t, "3", hp.Get(3))
	require.Equal(t, "4", hp.Get(4)) // 1 should have been evicted
	require.Equal(t, "1", hp.Get(1)) // 1 should be reopened
	require.Equal(t, "3", hp.Get(3)) // 3 is already in cache
	require.Equal(t, "4", hp.Get(4)) // 4 is already in cache

	require.Equal(t, 2, hp.openCount[1])
	require.Equal(t, 1, hp.openCount[2])
	require.Equal(t, 1, hp.openCount[3])
	require.Equal(t, 1, hp.openCount[4])

	require.Equal(t, 1, hp.closeCount[1])
	require.Equal(t, 1, hp.closeCount[2])
	require.Equal(t, 0, hp.closeCount[3])
	require.Equal(t, 0, hp.closeCount[4])

	hp.pool.CloseAll()

	require.Equal(t, 2, hp.openCount[1])
	require.Equal(t, 1, hp.openCount[2])
	require.Equal(t, 1, hp.openCount[3])
	require.Equal(t, 1, hp.openCount[4])

	require.Equal(t, 2, hp.closeCount[1])
	require.Equal(t, 1, hp.closeCount[2])
	require.Equal(t, 1, hp.closeCount[3])
	require.Equal(t, 1, hp.closeCount[4])

	require.Len(t, hp.pool.cache, 0)
	require.Equal(t, 0, hp.pool.queue.Len())
}

type handleAtomic struct {
	key    int
	isOpen atomic.Int64
}

func TestHandlePool_Concurrent(t *testing.T) {
	doTest := func(t *testing.T, numItems, numGoroutines, poolCapacity int) {
		// preallocate space for the items about to be opened, to avoid
		// imposing any extra synchronization on the pool implementation.
		items := make([]handleAtomic, numItems*numGoroutines)

		// every open will use the next slot in items, as indexed by itemN
		var itemN atomic.Int64

		opener := func(key int) *handleAtomic {
			n := itemN.Add(1) - 1
			h := &items[n]
			h.key = key
			opened := h.isOpen.Add(1)
			// avoid calling require.Equal for every check; it does a bunch of
			// runtime type checking
			if opened > 1 {
				t.Fatal("handle was already open")
			} else if opened < 1 {
				t.Fatal("handle was < 0??")
			}
			return h
		}

		closer := func(key int, h *handleAtomic) {
			closed := h.isOpen.Add(-1)
			if closed > 0 {
				t.Fatal("handle was > 1??")
			} else if closed < 0 {
				t.Fatal("handle closed when not open")
			}
		}

		pool := NewHandlePool[int, *handleAtomic](poolCapacity, opener, closer)

		// have each goroutine open each item in a random order.
		var group errgroup.Group
		for goroutineNum := 0; goroutineNum < numGoroutines; goroutineNum++ {
			accessOrder := make([]int, numItems)
			for i := 0; i < numItems; i++ {
				accessOrder[i] = i
			}
			rand.Shuffle(len(accessOrder), func(i, j int) {
				accessOrder[i], accessOrder[j] = accessOrder[j], accessOrder[i]
			})

			group.Go(func() error {
				for j := 0; j < numItems; j++ {
					if err := func() error {
						key := accessOrder[j]
						h, release := pool.Get(key)
						defer release()
						if h == nil {
							return errors.New("handle not found")
						}
						if h.key != key {
							return errors.New("bad key returned")
						}
						return nil
					}(); err != nil {
						return err
					}
				}
				return nil
			})
		}
		err := group.Wait()
		require.NoError(t, err)

		pool.CloseAll()

		// check that all opens were closed
		numOpens := itemN.Load()
		for i := int64(0); i < numOpens; i++ {
			if items[i].isOpen.Load() != 0 {
				t.Fatalf("handle %d not closed", i)
			}
		}
	}

	for _, test := range []struct {
		numItems, numGoroutines, poolCapacity int
	}{
		{10, 10, 1},
		{10, 10, 3},
		{10, 10, 10},
		{100, 100, 10},
		{1000, 100, 10},
	} {
		t.Run(fmt.Sprintf("%d-%d-%d", test.numItems, test.numGoroutines, test.poolCapacity), func(t *testing.T) {
			doTest(t, test.numItems, test.numGoroutines, test.poolCapacity)
		})
	}
}

func TestHandlePool_Delete(t *testing.T) {
	hp := newHandlePoolCounter(t, 3)
	require.Equal(t, "1", hp.Get(1))
	require.Equal(t, "2", hp.Get(2))
	require.Equal(t, "3", hp.Get(3))

	hp.pool.Delete(2)
	require.Equal(t, "1", hp.Get(1))
	require.Equal(t, "2", hp.Get(2)) // 2 should be reopened
	require.Equal(t, "3", hp.Get(3))

	require.Equal(t, 1, hp.openCount[1])
	require.Equal(t, 2, hp.openCount[2])
	require.Equal(t, 1, hp.openCount[3])
	require.Equal(t, 0, hp.closeCount[1])
	require.Equal(t, 1, hp.closeCount[2])
	require.Equal(t, 0, hp.closeCount[3])

	hp.pool.CloseAll()
	require.Equal(t, 1, hp.openCount[1])
	require.Equal(t, 2, hp.openCount[2])
	require.Equal(t, 1, hp.openCount[3])
	require.Equal(t, 1, hp.closeCount[1])
	require.Equal(t, 2, hp.closeCount[2])
	require.Equal(t, 1, hp.closeCount[3])

	require.Len(t, hp.pool.cache, 0)
	require.Equal(t, 0, hp.pool.queue.Len())
}
