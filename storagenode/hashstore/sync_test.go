// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/zeebo/assert"

	"storj.io/drpc/drpcsignal"
)

func TestMutex(t *testing.T) {
	mu := newMutex()
	closed := new(drpcsignal.Signal)
	ctx := context.Background()

	var state int
	var wg sync.WaitGroup

	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()

			assert.NoError(t, mu.Lock(ctx, closed))
			defer mu.Unlock()

			state++
		}()
	}

	wg.Wait()
	assert.Equal(t, state, 100)

	{ // canceled context should fail acquire
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		assert.Error(t, mu.Lock(ctx, closed))
	}

	{ // closed signal should fail acquire
		closed.Set(nil)
		assert.Error(t, mu.Lock(ctx, closed))
	}
}

func TestRWMutex(t *testing.T) {
	mu := newRWMutex()
	closed := new(drpcsignal.Signal)
	ctx := context.Background()

	var state int
	var wg sync.WaitGroup

	wg.Add(200)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()

			assert.NoError(t, mu.Lock(ctx, closed))
			defer mu.Unlock()

			state++
		}()
	}
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()

			assert.NoError(t, mu.RLock(ctx, closed))
			defer mu.RUnlock()

			runtime.KeepAlive(state)
		}()
	}

	wg.Wait()
	assert.Equal(t, state, 100)

	{ // canceled context should fail acquire
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		assert.Error(t, mu.Lock(ctx, closed))
		assert.Error(t, mu.RLock(ctx, closed))
	}

	{ // closed signal should fail acquire
		closed.Set(nil)
		assert.Error(t, mu.Lock(ctx, closed))
		assert.Error(t, mu.RLock(ctx, closed))
	}
}

func BenchmarkRWMutex(b *testing.B) {
	b.Run("Read", func(b *testing.B) {
		mu := newRWMutex()
		closed := new(drpcsignal.Signal)
		ctx := context.Background()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = mu.RLock(ctx, closed)
			mu.RUnlock()
		}
	})

	b.Run("Write", func(b *testing.B) {
		mu := newRWMutex()
		closed := new(drpcsignal.Signal)
		ctx := context.Background()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = mu.Lock(ctx, closed)
			mu.Unlock()
		}
	})
}
