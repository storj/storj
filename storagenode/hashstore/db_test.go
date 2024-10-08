// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestDB_BasicOperation(t *testing.T) {
	ctx := context.Background()

	db := newTestDB(t, 8, nil, nil)
	defer db.Close()

	var keys []Key

	// add enough keys to ensure a compaction.
	for i := 0; i < 1<<store_minTableSize*2; i++ {
		keys = append(keys, db.AssertCreate(time.Time{}))
	}

	for _, key := range keys {
		db.AssertRead(key)
	}

	// should still have all the keys after reopen.
	db.AssertReopen()
	for _, key := range keys {
		db.AssertRead(key)
	}

	// create and read should fail after close.
	db.Close()

	_, err := db.Read(ctx, newKey())
	assert.Error(t, err)

	_, err = db.Create(ctx, newKey(), time.Time{})
	assert.Error(t, err)
}

func TestDB_CompactionOnOpen(t *testing.T) {
	ctx := context.Background()

	db := newTestDB(t, 1, nil, nil)
	defer db.Close()

	// load up both the active and passive stores to somewhere between compact and max load.
	for db.active.Load() < (db_CompactLoad+db_MaxLoad)/2 {
		w, err := db.active.Create(ctx, newKey(), time.Time{})
		assert.NoError(t, err)
		assert.NoError(t, w.Close())
	}
	for db.passive.Load() < (db_CompactLoad+db_MaxLoad)/2 {
		w, err := db.passive.Create(ctx, newKey(), time.Time{})
		assert.NoError(t, err)
		assert.NoError(t, w.Close())
	}

	// reopening the store should cause passive to eventually be compacted.
	db.AssertReopen()

	for db.passive.Load() > db_CompactLoad {
		time.Sleep(time.Millisecond)
	}
}

func TestDB_SlowCompactionCreatesBackpressure(t *testing.T) {
	var done atomic.Bool
	throttle := make(chan struct{})

	db := newTestDB(t, 1, func(ctx context.Context, key Key, created time.Time) (bool, error) {
		<-throttle
		return false, nil
	}, nil)
	defer db.Close()

	// launch a goroutine that confirms that this test has a Create call blocked in waitOnState then
	// allows compaction to proceed.
	go func() {
		waitForGoroutine(
			"TestDB_SlowCompactionCreatesBackpressure",
			"Create",
			"waitOnState",
		)
		// signal that we can stop writing and allow compaction to proceed.
		done.Store(true)
		close(throttle)
	}()

	for !done.Load() {
		db.AssertCreate(time.Time{})
	}
}

func TestDB_CloseCancelsCompaction(t *testing.T) {
	var done atomic.Bool

	db := newTestDB(t, 1, blockOnContext, nil)
	defer db.Close()

	// launch a goroutine that confirms that this test has a Create call blocked in waitOnState then
	// allows compaction to proceed.
	go func() {
		waitForGoroutine(
			"TestDB_CloseCancelsCompaction",
			"Create",
			"waitOnState",
		)
		// signal that we can stop writing and close the database which should cancel the context
		// and allow compaction to proceed.
		done.Store(true)
		db.Close()
	}()

	for {
		w, err := db.Create(context.Background(), newKey(), time.Time{})
		if err == nil {
			assert.NoError(t, w.Close())
		} else {
			assert.True(t, done.Load())
			break
		}
	}
}

func TestDB_ContextCancelsCreate(t *testing.T) {
	var done atomic.Bool

	db := newTestDB(t, 1, blockOnContext, nil)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// launch a goroutine that confirms that this test has a Create call blocked in waitOnState then
	// allows compaction to proceed.
	go func() {
		waitForGoroutine(
			"TestDB_ContextCancelsCreate",
			"Create",
			"waitOnState",
		)
		// signal that we can stop writing and close the database which should cancel the context
		// and allow compaction to proceed.
		done.Store(true)
		cancel()
	}()

	for {
		w, err := db.Create(ctx, newKey(), time.Time{})
		if err == nil {
			assert.NoError(t, w.Close())
		} else {
			assert.That(t, errors.Is(err, context.Canceled))
			assert.True(t, done.Load())
			break
		}
	}
}

//
// benchmarks
//

func BenchmarkDB(b *testing.B) {
	ctx := context.Background()

	benchmarkSizes(b, "Create", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		db, err := New(b.TempDir(), 8, nil, nil, nil)
		assert.NoError(b, err)
		defer db.Close()

		b.SetBytes(int64(size))
		b.ReportAllocs()
		b.ResetTimer()

		now := time.Now()

		for i := 0; i < b.N; i++ {
			wr, err := db.Create(ctx, newKey(), time.Time{})
			assert.NoError(b, err)

			_, err = wr.Write(buf)
			assert.NoError(b, err)
			assert.NoError(b, wr.Close())
		}

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	})

	benchmarkSizes(b, "CreateParallel", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		db, err := New(b.TempDir(), runtime.GOMAXPROCS(0), nil, nil, nil)
		assert.NoError(b, err)
		defer db.Close()

		b.SetBytes(int64(size))
		b.ReportAllocs()
		b.ResetTimer()

		now := time.Now()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				wr, err := db.Create(ctx, newKey(), time.Time{})
				assert.NoError(b, err)

				_, err = wr.Write(buf)
				assert.NoError(b, err)
				assert.NoError(b, wr.Close())
			}
		})

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	})

	benchmarkSizes(b, "Read", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		db, err := New(b.TempDir(), 8, nil, nil, nil)
		assert.NoError(b, err)
		defer db.Close()

		var keys []Key
		for i := 0; i < 1<<store_minTableSize*2; i++ {
			key := newKey()
			keys = append(keys, key)

			wr, err := db.Create(ctx, key, time.Time{})
			assert.NoError(b, err)

			_, err = wr.Write(buf)
			assert.NoError(b, err)
			assert.NoError(b, wr.Close())
		}

		b.SetBytes(int64(size))
		b.ReportAllocs()
		b.ResetTimer()

		now := time.Now()

		for i := 0; i < b.N; i++ {
			r, err := db.Read(ctx, keys[mwc.Intn(len(keys))])
			assert.NoError(b, err)
			assert.NotNil(b, r)

			_, err = io.Copy(io.Discard, r)
			assert.NoError(b, r.Close())
			assert.NoError(b, err)
		}

		b.ReportMetric(float64(b.N)/time.Since(now).Seconds(), "pieces/sec")
	})
}
