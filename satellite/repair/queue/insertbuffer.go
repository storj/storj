// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"context"
)

// InsertBuffer exposes a synchronous API to buffer a batch of segments
// and insert them at once. Not threadsafe. Call Flush() before discarding.
//
//lint:ignore U1000 unused skeleton code
type InsertBuffer struct {
	queue     RepairQueue       //nolint
	batchSize int               //nolint
	batch     []*InjuredSegment //nolint
	// newInsertCallbacks contains callback called when the InjuredSegment
	// is flushed to the queue and it is determined that it wasn't already queued for repair.
	// This is made to collect metrics.
	newInsertCallbacks map[*InjuredSegment]func() //nolint
}

// Insert adds a segment to the batch of the next insert,
// and does a synchronous database insert when the batch size is reached.
// When it is determined that this segment is newly queued, firstInsertCallback is called.
// for the purpose of metrics.
//
//lint:ignore U1000 skeleton code
func (r *InsertBuffer) Insert(
	ctx context.Context,
	segment *InjuredSegment,
	newInsertCallback func(),
) (err error) {
	return err
}

// Flush inserts the remaining segments into the database.
//
//lint:ignore U1000 skeleton code
func (r *InsertBuffer) Flush(ctx context.Context) (err error) {
	return err
}
