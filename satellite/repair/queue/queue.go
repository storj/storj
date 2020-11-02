// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"context"
	"time"

	"storj.io/storj/satellite/internalpb"
)

// RepairQueue implements queueing for segments that need repairing.
// Implementation can be found at satellite/satellitedb/repairqueue.go.
//
// architecture: Database
type RepairQueue interface {
	// Insert adds an injured segment.
	Insert(ctx context.Context, s *internalpb.InjuredSegment, numHealthy int) (alreadyInserted bool, err error)
	// Select gets an injured segment.
	Select(ctx context.Context) (*internalpb.InjuredSegment, error)
	// Delete removes an injured segment.
	Delete(ctx context.Context, s *internalpb.InjuredSegment) error
	// Clean removes all segments last updated before a certain time
	Clean(ctx context.Context, before time.Time) (deleted int64, err error)
	// SelectN lists limit amount of injured segments.
	SelectN(ctx context.Context, limit int) ([]internalpb.InjuredSegment, error)
	// Count counts the number of segments in the repair queue.
	Count(ctx context.Context) (count int, err error)
}
