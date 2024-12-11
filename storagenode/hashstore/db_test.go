// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestDB_BasicOperation(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, nil, nil)
	defer db.Close()

	var keys []Key

	// add keys and ensure we can read them back
	for i := 0; i < 1000; i++ {
		keys = append(keys, db.AssertCreate())
	}
	for _, key := range keys {
		db.AssertRead(key)
	}

	// ensure the stats look like what we expect.
	stats, _, _ := db.Stats()
	t.Logf("%+v", stats)
	assert.Equal(t, stats.NumSet, 1000)
	assert.Equal(t, stats.LenSet, uint64(len(Key{})+RecordSize)*stats.NumSet)
	assert.That(t, stats.LenSet <= stats.LenLogs) // <= because of optimistic alignment

	// should still have all the keys after manual compaction
	db.AssertCompact()
	for _, key := range keys {
		db.AssertRead(key)
	}
	stats2, _, _ := db.Stats()
	assert.That(t, stats2.Compactions >= stats.Compactions+2)

	// should still have all the keys after reopen.
	db.AssertReopen()
	for _, key := range keys {
		db.AssertRead(key)
	}

	// reading a missing key should error
	_, err := db.Read(ctx, newKey())
	assert.Error(t, err)
	assert.That(t, errors.Is(err, fs.ErrNotExist))

	// create and read should fail after close.
	db.Close()

	_, err = db.Read(ctx, newKey())
	assert.Error(t, err)

	_, err = db.Create(ctx, newKey(), time.Time{})
	assert.Error(t, err)
}

func TestDB_TrashStats(t *testing.T) {
	db := newTestDB(t, alwaysTrash, nil)
	defer db.Close()

	// add keys until we are compacting, and then wait until we are not compacting.
	for {
		db.AssertCreate()
		stat, _, _ := db.Stats()
		if stat.Compacting {
			break
		}
	}

	require.Eventually(t, func() bool {
		stats, _, _ := db.Stats()
		return !stats.Compacting
	}, 1*time.Minute, time.Millisecond)

	// ensure the trash stats are updated.
	stats, _, _ := db.Stats()
	assert.That(t, stats.NumTrash > 0)
	assert.That(t, stats.LenTrash > 0)
	assert.That(t, stats.AvgTrash > 0)
	assert.That(t, stats.TrashPercent > 0)
}

func TestDB_TTLStats(t *testing.T) {
	db := newTestDB(t, nil, nil)
	defer db.Close()

	// create an entry with a ttl.
	db.AssertCreate(WithTTL(time.Now()))

	// ensure the ttl stats are updated.
	stats, _, _ := db.Stats()
	assert.Equal(t, stats.NumLogs, stats.NumLogsTTL)
	assert.Equal(t, stats.LenLogs, stats.LenLogsTTL)

	// create an entry without ttl.
	db.AssertCreate()

	// ensure the non-ttl stats are updated.
	stats, _, _ = db.Stats()
	assert.Equal(t, stats.NumLogs, 2*stats.NumLogsTTL)
	assert.Equal(t, stats.LenLogs, 2*stats.LenLogsTTL)
}

func TestDB_CompactionOnOpen(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t, nil, nil)
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

	db := newTestDB(t, func(ctx context.Context, key Key, created time.Time) bool {
		<-throttle
		return false
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
		db.AssertCreate()
	}
}

func TestDB_CloseCancelsCompaction(t *testing.T) {
	var done atomic.Bool

	db := newTestDB(t, blockOnContext, nil)
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

	db := newTestDB(t, blockOnContext, nil)
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

func TestDB_BackgroundCompaction(t *testing.T) {
	run := func(t *testing.T, getStore func(db *testDB) *Store) {
		db := newTestDB(t, nil, nil)
		defer db.Close()

		// while holding the db mutex so that no compactions can start, wait for the store to be in
		// a state where it is not compacting.
		db.mu.Lock()

		s := getStore(db)

		stats := func() StoreStats {
			for {
				if stats := s.Stats(); !stats.Compacting {
					return stats
				}
				time.Sleep(time.Millisecond)
			}
		}()

		// no compaction is going on and none can start, so we're safe to update the today callback
		// on the store data-race free.
		today := stats.Today + 2
		s.today = func() uint32 { return today }

		db.mu.Unlock()

		// trigger a check which should ensure that the store is eventually compacted.
		db.checkBackgroundCompactions()

		// sleep until the number of compactions increases
		for s.Stats().Compactions <= stats.Compactions {
			time.Sleep(time.Millisecond)
		}
	}

	t.Run("Active", func(t *testing.T) {
		run(t, func(db *testDB) *Store { return db.active })
	})
	t.Run("Passive", func(t *testing.T) {
		run(t, func(db *testDB) *Store { return db.passive })
	})
}

func TestDB_CompactCallWaitsForCurrentCompaction(t *testing.T) {
	var done atomic.Bool
	throttle := make(chan struct{})

	db := newTestDB(t, func(ctx context.Context, key Key, created time.Time) bool {
		done.Store(true)
		for range throttle {
		}
		return false
	}, nil)
	defer db.Close()

	// write entries until a background compaction has started.
	for !done.Load() {
		db.AssertCreate()
	}

	// wait for a Compact call to be blocked in select waiting for the previous compaction and then
	// allow the compaction to proceed.
	go func() {
		waitForGoroutine(
			"hashstore.(*DB).Compact",
			"[select]",
		)
		close(throttle)
	}()

	assert.NoError(t, db.Compact(context.Background()))
}

//
// benchmarks
//

func BenchmarkDB(b *testing.B) {
	ctx := context.Background()

	benchmarkSizes(b, "Create", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		db, err := New(b.TempDir(), nil, nil, nil)
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

		db, err := New(b.TempDir(), nil, nil, nil)
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

		db, err := New(b.TempDir(), nil, nil, nil)
		assert.NoError(b, err)
		defer db.Close()

		// write at most ~100MB of keys or 1000 keys, whichever is smaller. this keeps the benchmark
		// time reasonable.
		numKeys := 100 << 20 / (int(size) + 64)
		if numKeys > 1000 {
			numKeys = 1000
		}

		var keys []Key
		for i := 0; i < numKeys; i++ {
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
