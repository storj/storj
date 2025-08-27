// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/zeebo/assert"

	"storj.io/common/sync2"
	"storj.io/drpc/drpcsignal"
)

func TestMutex(t *testing.T) {
	mu := newMutex()
	closed := new(drpcsignal.Signal)
	ctx := t.Context()

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

func TestMutex_LockBlocksUntilCanceled(t *testing.T) {
	testCases := []struct {
		name       string
		cancelType string
	}{
		{name: "ContextCancel", cancelType: "context"},
		{name: "SignalClose", cancelType: "signal"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mu := newMutex()
			sig := new(drpcsignal.Signal)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			// Acquire the lock first
			mu.WaitLock()
			defer mu.Unlock()

			// Channel to receive the error from the second lock attempt
			errCh := make(chan error, 1)

			go func() {
				// Try to acquire another lock, which should block
				errCh <- mu.Lock(ctx, sig)
			}()

			// Wait for the goroutine to reach the blocking state
			waitForGoroutine(
				"TestMutex_LockBlocksUntilCanceled",
				"(*mutex).Lock(",
				"select",
			)

			if tc.cancelType == "context" {
				cancel() // Cancel the context after confirming we're blocked
			} else {
				sig.Set(nil) // Close the signal after confirming we're blocked
			}

			// We should get an error since we canceled the context or closed the signal
			assert.Error(t, <-errCh)
		})
	}
}

func TestRWMutex(t *testing.T) {
	run := func(t *testing.T, lifo bool) {
		mu := newRWMutex(0, lifo)
		closed := new(drpcsignal.Signal)
		ctx := t.Context()

		var state int
		var wg sync.WaitGroup
		const N = 100

		wg.Add(2 * N)
		for i := 0; i < N; i++ {
			go func() {
				defer wg.Done()

				assert.NoError(t, mu.Lock(ctx, closed))
				defer mu.Unlock()

				state++
			}()
		}
		for i := 0; i < N; i++ {
			go func() {
				defer wg.Done()

				assert.NoError(t, mu.RLock(ctx, closed))
				defer mu.RUnlock()

				runtime.KeepAlive(state)
			}()
		}

		wg.Wait()
		assert.Equal(t, state, N)

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

	t.Run("FIFO", func(t *testing.T) { run(t, false) })
	t.Run("LIFO", func(t *testing.T) { run(t, true) })
}

func TestRWMutex_LockBlocksUntilCanceled(t *testing.T) {
	testCases := []struct {
		name       string
		isReadLock bool
		cancelType string
	}{
		{name: "WriteLock_ContextCancel", isReadLock: false, cancelType: "context"},
		{name: "WriteLock_SignalClose", isReadLock: false, cancelType: "signal"},
		{name: "ReadLock_ContextCancel", isReadLock: true, cancelType: "context"},
		{name: "ReadLock_SignalClose", isReadLock: true, cancelType: "signal"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run := func(t *testing.T, lifo bool) {
				mu := newRWMutex(0, lifo)
				sig := new(drpcsignal.Signal)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				// Acquire write lock first (blocks both read and write locks)
				mu.WaitLock()
				defer mu.Unlock()

				// Create a channel to receive the error from the second lock attempt
				errCh := make(chan error, 1)

				go func() {
					var err error
					if tc.isReadLock {
						// Try to acquire a read lock, which should block
						err = mu.RLock(ctx, sig)
					} else {
						// Try to acquire another write lock, which should block
						err = mu.Lock(ctx, sig)
					}
					errCh <- err
				}()

				// Build the pattern for waitForGoroutine based on lock type
				lockFnPattern := "(*rwMutex).Lock("
				if tc.isReadLock {
					lockFnPattern = "(*rwMutex).RLock("
				}

				// Wait for the goroutine to reach the blocking state
				waitForGoroutine(
					"TestRWMutex_LockBlocksUntilCanceled",
					lockFnPattern,
					"(*rwMutex).lock(",
					"select",
				)

				if tc.cancelType == "context" {
					cancel() // Cancel the context after confirming we're blocked
				} else {
					sig.Set(nil) // Close the signal after confirming we're blocked
				}

				// We should get an error since we canceled the context or closed the signal
				assert.Error(t, <-errCh)
			}

			t.Run("FIFO", func(t *testing.T) { run(t, false) })
			t.Run("LIFO", func(t *testing.T) { run(t, true) })
		})
	}
}

func TestRWMutex_Semaphore_Failure(t *testing.T) {
	mu := newRWMutex(1, false)
	closed := new(drpcsignal.Signal)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// acquire one mutex
	assert.NoError(t, mu.RLock(ctx, closed))
	defer mu.RUnlock()

	go func() {
		// wait until we're blocked waiting for the mutex again
		waitForGoroutine(
			"TestRWMutex_Semaphore",
			"(*rwMutex).RLock(",
			"(*rwMutex).lock(",
			"select",
		)
		cancel()
	}()

	// acquire the mutex again
	assert.Error(t, mu.RLock(ctx, closed))
}

func TestRWMutex_Semaphore_Success(t *testing.T) {
	mu := newRWMutex(1, false)
	closed := new(drpcsignal.Signal)
	ctx := t.Context()

	// acquire one mutex
	assert.NoError(t, mu.RLock(ctx, closed))

	go func() {
		// wait until we're blocked waiting for the mutex again
		waitForGoroutine(
			"TestRWMutex_Semaphore",
			"(*rwMutex).RLock(",
			"(*rwMutex).lock(",
			"select",
		)
		mu.RUnlock()
	}()

	// acquire the mutex again
	assert.NoError(t, mu.RLock(ctx, closed))
}

func TestRWMutex_ReleaseCancelRace(t *testing.T) {
	mu := newRWMutex(0, false)
	sig := new(drpcsignal.Signal)

	for i := 0; i < 100; i++ {
		func() {
			// Create new context for each iteration
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			// Acquire write lock
			mu.WaitLock()

			// Create a wait group to track goroutines
			var wg sync2.WorkGroup

			wg.Go(func() {
				// Try to acquire the lock and unlock if successful
				if mu.Lock(ctx, sig) == nil {
					mu.Unlock()
				}
			})

			// Wait for goroutine to be blocked in select
			waitForGoroutine(
				"TestRWMutex_ReleaseCancelRace",
				"(*rwMutex).Lock(",
				"(*rwMutex).lock(",
				"select",
			)

			// Race between releasing mutex and canceling context, keeping track of them.
			wg.Go(mu.Unlock)
			wg.Go(cancel)

			// Wait for all the goroutines to finish before starting next iteration
			wg.Wait()
		}()
	}
}

func TestRWMutex_PendingWriteDoesntPreventMultipleReads(t *testing.T) {
	mu := newRWMutex(0, true)
	closed := new(drpcsignal.Signal)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// define a helper to wait until some state has been reached
	waitFor := func(cb func(m *rwMutex) bool) {
		for {
			mu.mu.Lock()
			valid := cb(mu)
			mu.mu.Unlock()
			if valid {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// acquire a mutex
	mu.WaitLock()

	type result struct {
		read bool
		err  error
	}

	// create a channel to hold all of the mutex acquisition results
	active := make(chan result, 11)

	// add and wait for a pending write lock
	go func() { active <- result{false, mu.Lock(ctx, closed)} }()
	waitFor(func(m *rwMutex) bool {
		return m.pendingWrites == 1
	})

	// add and wait for some pending read locks
	for i := 0; i < 10; i++ {
		go func() { active <- result{true, mu.RLock(ctx, closed)} }()
	}
	waitFor(func(m *rwMutex) bool {
		oldest, n := m.waiters.oldest, 0
		for oldest != nil {
			n++
			oldest = oldest.newer
		}
		return n == 11
	})

	// no locks should be acquired yet
	assert.Equal(t, len(active), 0)

	// release the write lock
	mu.Unlock()

	// all of the read locks should acquire
	for i := 0; i < 10; i++ {
		res := <-active
		assert.That(t, res.read)
		assert.NoError(t, res.err)
	}

	// unlocking all of the read locks should let the write lock acquire
	for i := 0; i < 10; i++ {
		mu.RUnlock()
	}

	res := <-active
	assert.That(t, !res.read)
	assert.NoError(t, res.err)
}

func BenchmarkRWMutex(b *testing.B) {
	b.Run("Read", func(b *testing.B) {
		mu := newRWMutex(0, false)
		closed := new(drpcsignal.Signal)
		ctx := b.Context()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = mu.RLock(ctx, closed)
			mu.RUnlock()
		}
	})

	b.Run("Write", func(b *testing.B) {
		mu := newRWMutex(0, false)
		closed := new(drpcsignal.Signal)
		ctx := b.Context()

		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = mu.Lock(ctx, closed)
			mu.Unlock()
		}
	})
}
