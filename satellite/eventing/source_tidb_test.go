// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
)

var errTest = errors.New("test error")

func TestOutboxDrainer(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	makeResult := func() *manualPendingResult {
		return &manualPendingResult{ready: make(chan struct{})}
	}
	confirm := func(r *manualPendingResult) {
		close(r.ready)
	}

	t.Run("drainReady harvests only confirmed entries", func(t *testing.T) {
		d := newOutboxDrainer()
		r1 := makeResult()
		r2 := makeResult()
		r3 := makeResult()

		d.add(1, r1)
		d.add(2, r2)
		d.add(3, r3)

		confirm(r1)
		confirm(r3)

		ids := d.drainReady()
		require.ElementsMatch(t, []int64{1, 3}, ids)
		require.Len(t, d.pending, 1)
		require.Equal(t, int64(2), d.pending[0].id)
	})

	t.Run("drainReady returns nil when nothing confirmed", func(t *testing.T) {
		d := newOutboxDrainer()
		r1 := makeResult()
		d.add(1, r1)

		ids := d.drainReady()
		require.Nil(t, ids)
		require.Len(t, d.pending, 1)
	})

	t.Run("drainOldest blocks until oldest confirmed", func(t *testing.T) {
		d := newOutboxDrainer()
		r1 := makeResult()
		r2 := makeResult()
		d.add(1, r1)
		d.add(2, r2)

		ctx.Go(func() error {
			time.Sleep(10 * time.Millisecond)
			confirm(r1)
			return nil
		})

		id, err := d.drainOldest(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(1), id)
		require.Len(t, d.pending, 1)
		require.Equal(t, int64(2), d.pending[0].id)
	})

	t.Run("drainOldest returns error on failed result", func(t *testing.T) {
		d := newOutboxDrainer()
		r := &manualPendingResult{ready: make(chan struct{}), err: errTest}
		d.add(1, r)
		confirm(r)

		_, err := d.drainOldest(ctx)
		require.ErrorIs(t, err, errTest)
	})

	t.Run("drainOldest panics on empty drainer", func(t *testing.T) {
		d := newOutboxDrainer()
		require.Panics(t, func() {
			_, _ = d.drainOldest(ctx) //nolint:errcheck
		})
	})

	t.Run("drainOldest respects context cancellation", func(t *testing.T) {
		d := newOutboxDrainer()
		r := makeResult()
		d.add(1, r)

		cancelCtx, cancel := context.WithCancel(ctx)
		ctx.Go(func() error {
			time.Sleep(10 * time.Millisecond)
			cancel()
			return nil
		})

		_, err := d.drainOldest(cancelCtx)
		require.ErrorIs(t, err, context.Canceled)
	})
}

// fakeBucketEventDB is an in-memory BucketEventDB where the test picks the
// row IDs, so it can model rows becoming visible out of ID order.
type fakeBucketEventDB struct {
	mu   sync.Mutex
	rows map[int64]metabase.BucketEvent
	// nextOnRead, when set, is the ID of a row added on every read, to model
	// new rows arriving all the time.
	nextOnRead int64
	// perRead is how many rows are added on every read when nextOnRead is
	// set. Zero means one.
	perRead int
}

func newFakeBucketEventDB() *fakeBucketEventDB {
	return &fakeBucketEventDB{rows: make(map[int64]metabase.BucketEvent)}
}

// insert makes a row visible. The ID is also stored as the object version, so
// the publish callback, which does not get the row ID, can tell rows apart.
func (db *fakeBucketEventDB) insert(id int64) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.insertLocked(id)
}

func (db *fakeBucketEventDB) insertLocked(id int64) {
	db.rows[id] = metabase.BucketEvent{ID: id, ObjectStream: metabase.ObjectStream{Version: metabase.Version(id)}}
}

func (db *fakeBucketEventDB) count() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	return len(db.rows)
}

func (db *fakeBucketEventDB) ReadBucketEventBatch(ctx context.Context, afterID int64, limit int) ([]metabase.BucketEvent, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.nextOnRead != 0 {
		for range max(db.perRead, 1) {
			db.insertLocked(db.nextOnRead)
			db.nextOnRead++
		}
	}
	var batch []metabase.BucketEvent
	for _, id := range slices.Sorted(maps.Keys(db.rows)) {
		if id > afterID && len(batch) < limit {
			batch = append(batch, db.rows[id])
		}
	}
	return batch, nil
}

func (db *fakeBucketEventDB) DeleteBucketEvents(ctx context.Context, ids []int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, id := range ids {
		delete(db.rows, id)
	}
	return nil
}

// deliveries counts how many times each row was handed to the publish callback.
type deliveries struct {
	mu     sync.Mutex
	counts map[int64]int
}

func (d *deliveries) add(event ChangeEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.counts[int64(event.Version)]++
}

func (d *deliveries) get() map[int64]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return maps.Clone(d.counts)
}

func TestTiDBEventSourceReader(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const pollInterval = time.Millisecond
	const batchSize = 10

	// start runs Listen until the test ends. fn returns the pending result
	// for each delivered event; without fn every event is confirmed at once.
	start := func(t *testing.T, db *fakeBucketEventDB, fn func(ChangeEvent) PendingResult) *deliveries {
		d := &deliveries{counts: map[int64]int{}}
		source := NewTiDBEventSource(zaptest.NewLogger(t), db, pollInterval, batchSize)
		cancelCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- source.Listen(cancelCtx, func(event ChangeEvent) (PendingResult, error) {
				d.add(event)
				if fn != nil {
					return fn(event), nil
				}
				return ImmediateResult(event.CommitTimestamp), nil
			})
		}()
		t.Cleanup(func() {
			cancel()
			require.NoError(t, <-done)
		})
		return d
	}

	waitEmpty := func(t *testing.T, db *fakeBucketEventDB) {
		require.Eventually(t, func() bool { return db.count() == 0 }, 5*time.Second, time.Millisecond)
	}

	t.Run("delivers rows that become visible out of ID order", func(t *testing.T) {
		db := newFakeBucketEventDB()
		d := start(t, db, nil)

		// A TiDB server with a higher AUTO_INCREMENT range commits first.
		db.insert(30001)
		waitEmpty(t, db)

		// Another TiDB server, with a lower range, commits afterwards.
		db.insert(1)
		db.insert(2)
		waitEmpty(t, db)

		// Let the reader rescan a few times to catch any duplicate delivery.
		time.Sleep(50 * pollInterval)
		require.Equal(t, map[int64]int{1: 1, 2: 1, 30001: 1}, d.get())
	})

	t.Run("delivers lower IDs while new rows keep arriving", func(t *testing.T) {
		// A new row with a high ID arrives before every read, so reads never
		// come back empty.
		db := newFakeBucketEventDB()
		db.nextOnRead = 30001
		d := start(t, db, nil)

		time.Sleep(20 * pollInterval)
		db.insert(1)
		require.Eventually(t, func() bool { return d.get()[1] == 1 }, 5*time.Second, time.Millisecond)
	})

	t.Run("delivers lower IDs while a backlog keeps every batch full", func(t *testing.T) {
		// A full batch of rows with high IDs arrives before every read, so the
		// reader never reaches the end of the outbox.
		db := newFakeBucketEventDB()
		db.nextOnRead = 30001
		db.perRead = batchSize
		d := start(t, db, nil)

		time.Sleep(20 * pollInterval)
		db.insert(1)
		require.Eventually(t, func() bool { return d.get()[1] == 1 }, 5*time.Second, time.Millisecond)
	})

	t.Run("does not hand out rows waiting for confirmation again", func(t *testing.T) {
		db := newFakeBucketEventDB()
		confirm := make(chan struct{})
		d := start(t, db, func(ChangeEvent) PendingResult {
			return &manualPendingResult{ready: confirm}
		})

		db.insert(1)
		db.insert(2)
		require.Eventually(t, func() bool { return len(d.get()) == 2 }, 5*time.Second, time.Millisecond)

		// The rows stay in the outbox while the reader rescans many times.
		time.Sleep(50 * pollInterval)
		require.Equal(t, map[int64]int{1: 1, 2: 1}, d.get())
		require.Equal(t, 2, db.count())

		close(confirm)
		waitEmpty(t, db)
	})

	t.Run("pages through a backlog larger than one batch", func(t *testing.T) {
		db := newFakeBucketEventDB()
		want := map[int64]int{}
		for id := int64(1); id <= 3*batchSize+5; id++ {
			db.insert(id)
			want[id] = 1
		}
		d := start(t, db, nil)
		waitEmpty(t, db)
		require.Equal(t, want, d.get())
	})
}

func TestInflightIDs(t *testing.T) {
	addNew := func(f *inflightIDs, ids ...int64) []int64 {
		var batch []metabase.BucketEvent
		for _, id := range ids {
			batch = append(batch, metabase.BucketEvent{ID: id})
		}
		var got []int64
		for _, r := range f.addNew(batch) {
			got = append(got, r.ID)
		}
		return got
	}

	f := newInflightIDs()
	f.startRead()
	require.Equal(t, []int64{1, 2}, addNew(f, 1, 2))

	// Rows in flight are not handed out again.
	f.startRead()
	require.Equal(t, []int64{3}, addNew(f, 1, 2, 3))

	// A read that started before the delete committed can still return the
	// deleted row; it must not be handed out again.
	f.startRead()
	f.remove([]int64{1})
	require.Equal(t, []int64{4}, addNew(f, 1, 2, 3, 4))

	// The next read cannot see the deleted row anymore, so its ID is released.
	f.startRead()
	require.Equal(t, []int64{1}, addNew(f, 1))
}
