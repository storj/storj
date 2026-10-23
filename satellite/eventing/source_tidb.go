// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/metabase"
)

// rescanPolls is how many poll intervals pass at least between two scans of
// the outbox from the first row. A scan from the first row makes TiKV step
// over every recently deleted row that is not garbage collected yet, so it
// should not run on every poll.
const rescanPolls = 10

// BucketEventDB is the subset of metabase.TiDBAdapter used by TiDBEventSource.
// It is satisfied by *metabase.TiDBAdapter.
type BucketEventDB interface {
	ReadBucketEventBatch(ctx context.Context, afterID int64, limit int) ([]metabase.BucketEvent, error)
	DeleteBucketEvents(ctx context.Context, ids []int64) error
}

// TiDBEventSource implements EventSource by polling the bucket_eventing_outbox
// table. It uses three goroutines: a reader that fetches batches from the
// outbox, a publisher that decodes each row and calls fn, and a drainer that
// collects PendingResults, deletes confirmed batch, and applies backpressure
// when Pub/Sub is slow.
//
// Rows are deleted only after their publish is confirmed, so delivery is
// at-least-once (a crash between publish and delete redelivers on restart).
type TiDBEventSource struct {
	log          *zap.Logger
	db           BucketEventDB
	pollInterval time.Duration
	batchSize    int
}

// NewTiDBEventSource creates a TiDBEventSource.
func NewTiDBEventSource(log *zap.Logger, db BucketEventDB, pollInterval time.Duration, batchSize int) *TiDBEventSource {
	return &TiDBEventSource{
		log:          log,
		db:           db,
		pollInterval: pollInterval,
		batchSize:    batchSize,
	}
}

// Listen starts the three-goroutine outbox polling loop and calls fn for each
// decoded ChangeEvent. Blocks until ctx is cancelled or a permanent error occurs.
func (s *TiDBEventSource) Listen(ctx context.Context, fn func(ChangeEvent) (PendingResult, error)) error {
	// Capacity 1 lets the reader pre-fetch one batch while the publisher is
	// processing the previous one, without keeping more than two batches in
	// memory at once.
	batchCh := make(chan []metabase.BucketEvent, 1)
	// Buffer of batchSize lets the publisher keep sending while the drainer is
	// executing DeleteBucketEvents, without accumulating more than one batch of
	// unconfirmed entries beyond the drainer's flush threshold.
	pendingCh := make(chan pendingEntry, s.batchSize)
	inflight := newInflightIDs()

	s.log.Info("Starting TiDB outbox processor")

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(batchCh)
		return s.runReader(ctx, batchCh, inflight)
	})
	g.Go(func() error {
		defer close(pendingCh)
		return s.runPublisher(ctx, batchCh, pendingCh, fn)
	})
	g.Go(func() error {
		return s.runDrainer(ctx, pendingCh, inflight)
	})

	err := g.Wait()
	if err != nil && !errs.Is(err, context.Canceled) {
		s.log.Error("TiDB outbox processor exited with error", zap.Error(err))
		return err
	}

	s.log.Info("TiDB outbox processor exited")

	return nil
}

func (s *TiDBEventSource) runReader(ctx context.Context, batchCh chan<- []metabase.BucketEvent, inflight *inflightIDs) error {
	// afterID pages through the outbox; it is not a watermark. IDs are not in
	// commit order: TiDB gives every tidb-server its own AUTO_INCREMENT range,
	// and a row becomes visible only at COMMIT, so a row can appear with an ID
	// below rows that were already read. When the reader reaches the end of
	// the outbox it starts over from the first row (see rescanPolls), or else
	// continues from the highest ID seen. The rescan also interrupts a
	// backlog that keeps every batch full, so late rows are not held back
	// until the backlog is gone.
	var afterID, maxSeenID int64
	lastRescan := time.Now()
	for {
		inflight.startRead()
		batch, err := s.db.ReadBucketEventBatch(ctx, afterID, s.batchSize)
		if err != nil {
			return err
		}

		reachedEnd := len(batch) < s.batchSize
		prevMaxSeenID := maxSeenID
		if len(batch) > 0 {
			maxSeenID = max(maxSeenID, batch[len(batch)-1].ID)
		}
		switch {
		case time.Since(lastRescan) >= rescanPolls*s.pollInterval:
			afterID, lastRescan = 0, time.Now()
		case !reachedEnd:
			afterID = batch[len(batch)-1].ID
		default:
			afterID = maxSeenID
		}

		batch = inflight.addNew(batch)

		// Near batchSize frequently indicates a persistent outbox backlog.
		mon.IntVal("eventing_tidb_outbox_batch_size").Observe(int64(len(batch)))

		// Rows not in flight with an ID below one already seen committed late.
		var lateRows int64
		for _, r := range batch {
			if r.ID < prevMaxSeenID {
				lateRows++
			}
		}
		mon.Counter("eventing_tidb_outbox_late_rows").Inc(lateRows)

		if len(batch) > 0 {
			s.log.Debug("Fetched outbox batch", zap.Int("size", len(batch)), zap.Int64("last_id", batch[len(batch)-1].ID))

			// 0 means the publisher is the bottleneck (waiting for reader);
			// 1 means the reader is the bottleneck (waiting for publisher).
			mon.IntVal("eventing_tidb_outbox_batch_channel_fill").Observe(int64(len(batchCh)))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case batchCh <- batch:
			}
		}

		if reachedEnd {
			s.log.Debug("Reached the end of the outbox, waiting for next poll")
			if !sync2.Sleep(ctx, s.pollInterval) {
				return ctx.Err()
			}
		}
	}
}

func (s *TiDBEventSource) runPublisher(ctx context.Context, batchCh <-chan []metabase.BucketEvent, pendingCh chan<- pendingEntry, fn func(ChangeEvent) (PendingResult, error)) error {
	for batch := range batchCh {
		for _, r := range batch {
			event := ChangeEvent{
				EventName:       r.EventName,
				ObjectStream:    r.ObjectStream,
				TotalPlainSize:  r.TotalPlainSize,
				CommitTimestamp: r.CreatedAt,
			}

			result, err := fn(event)
			if err != nil {
				return err
			}

			// 0 means the drainer is the bottleneck (waiting for publisher);
			// batchSize means the publisher is the bottleneck (waiting for drainer).
			mon.IntVal("eventing_tidb_outbox_pending_channel_fill").Observe(int64(len(pendingCh)))

			// TODO: replace with a lock-free SPSC queue (similar to the combiner
			// queue) if channel overhead becomes a bottleneck at high throughput.
			select {
			case pendingCh <- pendingEntry{id: r.ID, result: result}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return ctx.Err()
}

func (s *TiDBEventSource) runDrainer(ctx context.Context, pendingCh <-chan pendingEntry, inflight *inflightIDs) error {
	drainer := newOutboxDrainer()
	// Flush confirmed IDs on the same cadence as the reader polls, so the
	// outbox drains promptly even when the publisher is idle between batches.
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	// deleteRows releases the IDs from inflight only after the rows are
	// deleted, so the reader cannot hand out a row it can still see.
	deleteRows := func(ids []int64) error {
		if err := s.db.DeleteBucketEvents(ctx, ids); err != nil {
			return err
		}
		inflight.remove(ids)
		return nil
	}

	for {
		select {
		case <-ticker.C:
			if err := deleteRows(drainer.drainReady()); err != nil {
				return err
			}
		case entry, ok := <-pendingCh:
			if !ok {
				// Publisher has exited and closed pendingCh.
				return ctx.Err()
			}
			drainer.add(entry.id, entry.result)
			if len(drainer.pending) >= s.batchSize {
				s.log.Debug("Drainer queue full, applying backpressure", zap.Int("pending", len(drainer.pending)))
				// Queue is full — wait for the oldest entry to confirm, then
				// harvest whatever else confirmed in the meantime.
				oldestID, err := drainer.drainOldest(ctx)
				if err != nil {
					return err
				}
				ids := append(drainer.drainReady(), oldestID)
				if err := deleteRows(ids); err != nil {
					return err
				}
			}
		}
	}
}

// outboxDrainer collects PendingResults and their associated outbox row IDs,
// opportunistically draining already-confirmed results after each add.
type outboxDrainer struct {
	pending []pendingEntry
}

type pendingEntry struct {
	id     int64
	result PendingResult
}

func newOutboxDrainer() *outboxDrainer {
	return &outboxDrainer{}
}

// add appends a result to the pending queue.
func (d *outboxDrainer) add(id int64, result PendingResult) {
	d.pending = append(d.pending, pendingEntry{id: id, result: result})
}

// drainReady non-blockingly harvests entries whose Ready channel is already
// closed and returns their IDs for deletion.
func (d *outboxDrainer) drainReady() []int64 {
	var ids []int64
	remaining := d.pending[:0]
	for _, e := range d.pending {
		select {
		case <-e.result.Ready():
			ids = append(ids, e.id)
		default:
			remaining = append(remaining, e)
		}
	}
	d.pending = remaining
	return ids
}

// drainOldest blocks until the oldest pending entry is confirmed, removes it,
// and returns its ID. Used to apply backpressure when the queue is full.
// Panics if the queue is empty.
func (d *outboxDrainer) drainOldest(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(d.pending) == 0 {
		panic("drainOldest called on empty drainer")
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-d.pending[0].result.Ready():
	}
	e := d.pending[0]
	d.pending = d.pending[1:]
	if err := e.result.Get(ctx); err != nil {
		return 0, err
	}
	return e.id, nil
}

// inflightIDs tracks outbox rows that the reader has handed out and the
// drainer has not deleted yet. Its size is bounded by the backpressure to a
// few batches.
type inflightIDs struct {
	mu  sync.Mutex
	ids map[int64]struct{}
	// released holds IDs of deleted rows. They stay in ids until the next read
	// starts, because a read that started before the DELETE committed can
	// still return them. This relies on the next read seeing the DELETE,
	// which holds as long as outbox reads don't use stale or follower reads.
	released []int64
}

func newInflightIDs() *inflightIDs {
	return &inflightIDs{ids: make(map[int64]struct{})}
}

// startRead must be called before each outbox read.
func (f *inflightIDs) startRead() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.released {
		delete(f.ids, id)
	}
	f.released = f.released[:0]
}

// addNew returns the rows that are not in flight and marks them as in flight.
func (f *inflightIDs) addNew(batch []metabase.BucketEvent) []metabase.BucketEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	fresh := batch[:0]
	for _, r := range batch {
		if _, ok := f.ids[r.ID]; ok {
			continue
		}
		f.ids[r.ID] = struct{}{}
		fresh = append(fresh, r)
	}
	return fresh
}

// remove releases IDs whose rows were deleted from the outbox.
func (f *inflightIDs) remove(ids []int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, ids...)
}
