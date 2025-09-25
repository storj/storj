// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestDB_BasicOperation(t *testing.T) {
	forAllTables(t, testDB_BasicOperation)
}
func testDB_BasicOperation(t *testing.T, cfg Config) {
	ctx := t.Context()
	db := newTestDB(t, cfg, nil, nil)
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
	assert.Equal(t, stats.LenSet, stats.LenLogs)

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

	// create, read and compact should fail after close.
	db.Close()

	_, err = db.Read(ctx, newKey())
	assert.Error(t, err)

	_, err = db.Create(ctx, newKey(), time.Time{})
	assert.Error(t, err)

	err = db.Compact(ctx)
	assert.Error(t, err)
}

func TestDB_ConcurrentOperation(t *testing.T) {
	forAllTables(t, testDB_ConcurrentOperation)
}
func testDB_ConcurrentOperation(t *testing.T, cfg Config) {
	cfg.Compaction.MaxLogSize = 1 << 10 // 1KiB

	db := newTestDB(t, cfg, nil, nil)
	defer db.Close()

	procs := runtime.GOMAXPROCS(-1)
	keysCh := make(chan []Key, procs)
	for range procs {
		go func() {
			var keys []Key
			for i := 0; ; i++ {
				// stop when we have ~100 log files and, if not short, compaction has happened
				stats, _, _ := db.Stats()
				if stats.NumLogs >= 100 && (testing.Short() || stats.Compactions > 0) {
					break
				}

				keys = append(keys, db.AssertCreate())

				// add about 10% concurrent reads
				if mwc.Intn(10) == 0 {
					db.AssertRead(keys[mwc.Intn(len(keys))])
				}
			}
			keysCh <- keys
		}()
	}

	// collect all the keys created by the goroutines.
	var allKeys []Key
	for range procs {
		allKeys = append(allKeys, <-keysCh...)
	}

	// ensure we can read all the keys back.
	for _, key := range allKeys {
		db.AssertRead(key)
	}

	// ensure we can still read all the keys back after reopen.
	db.AssertReopen()
	for _, key := range allKeys {
		db.AssertRead(key)
	}
}

func TestDB_TrashStats(t *testing.T) {
	forAllTables(t, testDB_TrashStats)
}
func testDB_TrashStats(t *testing.T, cfg Config) {
	db := newTestDB(t, cfg, alwaysTrash, nil)
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

func TestDB_ReadAllPossibleStates(t *testing.T) {
	forAllTables(t, testDB_ReadAllPossibleStates)
}
func testDB_ReadAllPossibleStates(t *testing.T, cfg Config) {
	const (
		createActive = iota
		createPassive
		compact
	)

	cases := []struct {
		name  string
		setup []int
	}{
		{"ActiveExist_PassiveNotExist", []int{createActive}},
		{"ActiveTrash_PassiveNotExist", []int{createActive, compact}},

		{"ActiveNotExist_PassiveExist", []int{createPassive}},
		{"ActiveExist_PassiveExist", []int{createActive, createPassive}},
		{"ActiveTrash_PassiveExist", []int{createActive, compact, createPassive}},

		{"ActiveNotExist_PassiveTrash", []int{createPassive, compact}},
		{"ActiveExist_PassiveTrash", []int{createPassive, compact, createActive}},
		{"ActiveTrash_PassiveTrash", []int{createPassive, createActive, compact}},
	}

	runCase := func(t *testing.T, setup []int) {
		db := newTestDB(t, cfg, alwaysTrash, nil)
		defer db.Close()
		key := newKey()

		// keep track of which store starts off as active so that we can ensure we are creating and
		// reading records from the correct store.
		active := db.active
		makeActive := func() {
			if active != db.active {
				db.swapStoresLocked()
			}
		}
		makePassive := func() {
			if active == db.active {
				db.swapStoresLocked()
			}
		}

		for _, op := range setup {
			switch op {
			case createActive:
				makeActive()
				db.AssertCreate(WithKey(key))

			case createPassive:
				makePassive()
				db.AssertCreate(WithKey(key))

			case compact:
				db.AssertCompact()
			}
		}

		makeActive()
		db.AssertRead(key)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { runCase(t, tc.setup) })
	}
}

func TestDB_TTLStats(t *testing.T) {
	forAllTables(t, testDB_TTLStats)
}
func testDB_TTLStats(t *testing.T, cfg Config) {
	db := newTestDB(t, cfg, nil, nil)
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
	forAllTables(t, testDB_CompactionOnOpen)
}
func testDB_CompactionOnOpen(t *testing.T, cfg Config) {
	ctx := t.Context()
	db := newTestDB(t, cfg, nil, nil)
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
	forAllTables(t, testDB_SlowCompactionCreatesBackpressure)
}
func testDB_SlowCompactionCreatesBackpressure(t *testing.T, cfg Config) {
	var done atomic.Bool
	throttle := make(chan struct{})

	db := newTestDB(t, cfg, func(ctx context.Context, key Key, created time.Time) bool {
		<-throttle
		return false
	}, nil)
	defer db.Close()

	// launch a goroutine that confirms that this test has a Create call blocked in waitOnState then
	// allows compaction to proceed.
	go func() {
		waitForGoroutine(
			"testDB_SlowCompactionCreatesBackpressure",
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
	forAllTables(t, testDB_CloseCancelsCompaction)
}
func testDB_CloseCancelsCompaction(t *testing.T, cfg Config) {
	var done atomic.Bool

	db := newTestDB(t, cfg, blockOnContext, nil)
	defer db.Close()

	// launch a goroutine that confirms that this test has a Create call blocked in waitOnState then
	// allows compaction to proceed.
	go func() {
		waitForGoroutine(
			"testDB_CloseCancelsCompaction",
			"Create",
			"waitOnState",
		)
		// signal that we can stop writing and close the database which should cancel the context
		// and allow compaction to proceed.
		done.Store(true)
		db.Close()
	}()

	for {
		w, err := db.Create(t.Context(), newKey(), time.Time{})
		if err == nil {
			assert.NoError(t, w.Close())
		} else {
			assert.True(t, done.Load())
			break
		}
	}
}

func TestDB_ContextCancelsCreate(t *testing.T) {
	forAllTables(t, testDB_ContextCancelsCreate)
}
func testDB_ContextCancelsCreate(t *testing.T, cfg Config) {
	var done atomic.Bool

	db := newTestDB(t, cfg, blockOnContext, nil)
	defer db.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// launch a goroutine that confirms that this test has a Create call blocked in waitOnState then
	// allows compaction to proceed.
	go func() {
		waitForGoroutine(
			"testDB_ContextCancelsCreate",
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
	forAllTables(t, testDB_BackgroundCompaction)
}
func testDB_BackgroundCompaction(t *testing.T, cfg Config) {
	run := func(t *testing.T, getStore func(db *testDB) *Store) {
		db := newTestDB(t, cfg, nil, nil)
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
		today := stats.Today + 1
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
	forAllTables(t, testDB_CompactCallWaitsForCurrentCompaction)
}
func testDB_CompactCallWaitsForCurrentCompaction(t *testing.T, cfg Config) {
	var done atomic.Bool
	throttle := make(chan struct{})

	db := newTestDB(t, cfg, func(ctx context.Context, key Key, created time.Time) bool {
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

	assert.NoError(t, db.Compact(t.Context()))
}

//
// benchmarks
//

func BenchmarkDB(b *testing.B) {
	forAllTables(b, benchmarkDB)
}
func benchmarkDB(b *testing.B, cfg Config) {
	ctx := b.Context()

	benchmarkSizes(b, "Create", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		db, err := New(ctx, cfg, b.TempDir(), "", nil, nil, nil)
		assert.NoError(b, err)
		defer assertClose(b, db)

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

	benchmarkSizes(b, "Read", func(b *testing.B, size uint64) {
		buf := make([]byte, size)
		_, _ = mwc.Rand().Read(buf)

		db, err := New(ctx, cfg, b.TempDir(), "", nil, nil, nil)
		assert.NoError(b, err)
		defer assertClose(b, db)

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
