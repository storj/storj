// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package changestream

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// pendingDrainSize is the maximum number of unconfirmed pending results to
// accumulate before blocking to drain. This bounds memory usage and applies
// backpressure when Pub/Sub is slow.
const pendingDrainSize = 100

// drainReadyInterval is how often the background goroutine drains confirmed
// results while the partition is idle (between records).
const drainReadyInterval = 100 * time.Millisecond

// watermarkUpdater is the subset of notifyingBatcher used by partitionDrainer,
// allowing it to be replaced with a test stub.
type watermarkUpdater interface {
	UpdatePartitionWatermark(partitionToken string, watermark time.Time)
}

// partitionDrainer manages the pending result queue for a single partition.
//
// drainReady is safe to call concurrently with add/drainAll — it only calls
// Get on results whose Ready() channel is already closed, so it never blocks
// while holding the mutex.
//
// drainAll blocks on Get and is only called from the record callback goroutine,
// never from the background drainReady ticker.
type partitionDrainer struct {
	mu             sync.Mutex
	log            *zap.Logger
	feedName       string
	partitionToken string
	watermarks     watermarkUpdater
	pending        []PendingResult
}

// newPartitionDrainer creates a partitionDrainer and starts a background
// goroutine that drains confirmed results periodically while the partition is
// idle between records. The caller must call the returned stop function when
// done to stop the background goroutine.
func newPartitionDrainer(ctx context.Context, log *zap.Logger, feedName, partitionToken string, watermarks watermarkUpdater) (_ *partitionDrainer, cancel func()) {
	d := &partitionDrainer{
		log:            log,
		feedName:       feedName,
		partitionToken: partitionToken,
		watermarks:     watermarks,
	}

	bgCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(drainReadyInterval)
		defer ticker.Stop()

		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				// Ignore errors — they will also surface in the record callback.
				_ = d.drainReady(bgCtx)
			}
		}
	}()

	return d, cancel
}

// add appends a result to the pending queue, opportunistically drains confirmed
// results, and blocks if the queue exceeds pendingDrainSize.
func (d *partitionDrainer) add(ctx context.Context, result PendingResult) (err error) {
	defer mon.Task()(&ctx)(&err)

	d.mu.Lock()
	d.pending = append(d.pending, result)
	d.mu.Unlock()

	if err := d.drainReady(ctx); err != nil {
		return err
	}

	d.mu.Lock()
	over := len(d.pending) >= pendingDrainSize
	d.mu.Unlock()

	if over {
		return d.drainAll(ctx)
	}

	return nil
}

// drainReady confirms all leading pending results that are already ready,
// without blocking. Safe to call concurrently with add and drainAll.
func (d *partitionDrainer) drainReady(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	d.mu.Lock()
	defer d.mu.Unlock()

	var highWatermark time.Time
	i := 0
	for i < len(d.pending) {
		ready := false
		select {
		case <-d.pending[i].Ready():
			ready = true
		default:
		}
		if !ready {
			// Oldest result not ready yet — stop.
			break
		}
		// Get is non-blocking here because Ready() is already closed.
		if err := d.pending[i].Get(ctx); err != nil {
			// Remove the failed result and all confirmed predecessors. Without this,
			// the background goroutine could re-process the failed result (whose
			// Ready() channel is still closed) before stopDrainer() is called,
			// silently advancing the watermark past a failed event.
			d.pending = d.pending[:copy(d.pending, d.pending[i+1:])]
			if !highWatermark.IsZero() {
				d.watermarks.UpdatePartitionWatermark(d.partitionToken, highWatermark)
			}
			return err
		}
		if ts := d.pending[i].Timestamp(); ts.After(highWatermark) {
			highWatermark = ts
		}
		i++
	}

	if i == 0 {
		return nil
	}

	d.pending = d.pending[:copy(d.pending, d.pending[i:])]

	if !highWatermark.IsZero() {
		d.log.Debug("Draining pending results. Updating partition watermark",
			zap.String("change_stream", d.feedName),
			zap.String("partition_token", d.partitionToken),
			zap.Int("drained", i),
			zap.Time("watermark", highWatermark))
		d.watermarks.UpdatePartitionWatermark(d.partitionToken, highWatermark)
	}

	return nil
}

// drainAll blocks until all pending results are confirmed and advances
// the watermark. Used on heartbeat and partition end.
func (d *partitionDrainer) drainAll(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	d.mu.Lock()
	pending := d.pending
	d.pending = nil
	d.mu.Unlock()

	var highWatermark time.Time
	for _, result := range pending {
		if err := result.Get(ctx); err != nil {
			// Advance the watermark for results already confirmed before the error.
			if !highWatermark.IsZero() {
				d.watermarks.UpdatePartitionWatermark(d.partitionToken, highWatermark)
			}
			return err
		}
		if ts := result.Timestamp(); ts.After(highWatermark) {
			highWatermark = ts
		}
	}

	if !highWatermark.IsZero() {
		d.log.Debug("Draining pending results. Updating partition watermark",
			zap.String("change_stream", d.feedName),
			zap.String("partition_token", d.partitionToken),
			zap.Int("drained", len(pending)),
			zap.Time("watermark", highWatermark))
		d.watermarks.UpdatePartitionWatermark(d.partitionToken, highWatermark)
	}

	return nil
}
