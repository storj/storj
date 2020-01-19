// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cache

import (
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
)

func TestCache_LRU(t *testing.T) {
	cache := New(Options{Capacity: 2})
	check := newChecker(t, cache)

	check("a", 1)
	check("a", 1)
	check("b", 2)
	check("a", 2)
	check("c", 3)
	check("b", 4)
	check("c", 4)
	check("a", 5)
}

func TestCache_Expires(t *testing.T) {
	cache := New(Options{Capacity: 2, Expiration: time.Nanosecond})
	check := newChecker(t, cache)

	check("a", 1)
	time.Sleep(time.Second)
	check("a", 2)
}

func TestCache_Fuzz(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cache := New(Options{Capacity: 2, Expiration: 100 * time.Millisecond})
	keys := "abcdefghij"

	var ops uint64
	procs := runtime.GOMAXPROCS(-1)

	for i := 0; i < procs; i++ {
		ctx.Go(func() error {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for {
				if atomic.AddUint64(&ops, 1) > 1000000 {
					return nil
				}

				shouldErr := rng.Intn(10) == 0
				ran := false
				kidx := rng.Intn(len(keys))
				key := keys[kidx : kidx+1]

				value, err := cache.Get(key, func() (interface{}, error) {
					ran = true
					if shouldErr {
						return nil, errs.New("random error")
					}
					return key, nil
				})

				if ran {
					if shouldErr && err == nil {
						return errs.New("should have errored and did not")
					}
					if !shouldErr && err != nil {
						return errs.New("should not have errored but did")
					}
				}
				if value != key && !(ran && shouldErr) {
					return errs.New("expected %q but got %q", key, value)
				}
			}
		})
	}

	ctx.Wait()
}

//
// helper
//

type checker struct {
	t     *testing.T
	cache *ExpiringLRU
	calls int
}

func newChecker(t *testing.T, cache *ExpiringLRU) func(string, int) {
	return (&checker{t: t, cache: cache}).Check
}

func (c *checker) makeCallback(v interface{}) func() (interface{}, error) {
	return func() (interface{}, error) {
		c.calls++
		return v, nil
	}
}

func (c *checker) Check(key string, calls int) {
	value, err := c.cache.Get(key, c.makeCallback(key))
	require.Equal(c.t, c.calls, calls)
	require.Equal(c.t, value, key)
	require.NoError(c.t, err)
}
