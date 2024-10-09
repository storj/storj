// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
)

var mon = monkit.Package()

// InsertBuffer exposes a synchronous API to buffer a batch of segments
// and insert them at once. Not threadsafe. Call Flush() before discarding.
type InsertBuffer struct {
	queue     RepairQueue
	batchSize int

	batch []*InjuredSegment
	// newInsertCallbacks contains callback called when the InjuredSegment
	// is flushed to the queue and it is determined that it wasn't already queued for repair.
	// This is made to collect metrics.
	newInsertCallbacks map[*InjuredSegment]func()
}

// NewInsertBuffer wraps a RepairQueue with buffer logic.
func NewInsertBuffer(
	queue RepairQueue,
	batchSize int,
) *InsertBuffer {
	insertBuffer := InsertBuffer{
		queue:              queue,
		batchSize:          batchSize,
		batch:              make([]*InjuredSegment, 0, batchSize),
		newInsertCallbacks: make(map[*InjuredSegment]func()),
	}

	return &insertBuffer
}

// Insert adds a segment to the batch of the next insert,
// and does a synchronous database insert when the batch size is reached.
// When it is determined that this segment is newly queued, firstInsertCallback is called.
// for the purpose of metrics.
func (r *InsertBuffer) Insert(
	ctx context.Context,
	segment *InjuredSegment,
	newInsertCallback func(),
) (err error) {
	defer mon.Task()(&ctx)(&err)

	r.batch = append(r.batch, segment)
	r.newInsertCallbacks[segment] = newInsertCallback

	if len(r.batch) < r.batchSize {
		return nil
	}

	return r.Flush(ctx)
}

// Flush inserts the remaining segments into the database.
func (r *InsertBuffer) Flush(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	newlyInsertedSegments, err := r.queue.InsertBatch(ctx, r.batch)
	if err != nil {
		return err
	}

	for _, segment := range newlyInsertedSegments {
		callback := r.newInsertCallbacks[segment]
		if callback != nil {
			callback()
		}
	}

	r.clearInternals()

	return nil
}

func (r *InsertBuffer) clearInternals() {
	// make room for the next batch
	r.batch = r.batch[:0]

	for key := range r.newInsertCallbacks {
		delete(r.newInsertCallbacks, key)
	}
}
