// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"context"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// InjuredSegment contains information about segment which
// should be repaired.
type InjuredSegment struct {
	StreamID uuid.UUID
	Position metabase.SegmentPosition

	SegmentHealth float64
	AttemptedAt   *time.Time
	UpdatedAt     time.Time
	InsertedAt    time.Time

	Placement storj.PlacementConstraint

	NumNormalizedHealthy     int16
	NumNormalizedRetrievable int16
	NumOutOfPlacement        int16
}

// Stat contains information about a segment of repair queue.
type Stat struct {
	Count            int
	Placement        storj.PlacementConstraint
	MaxInsertedAt    time.Time
	MinInsertedAt    time.Time
	MaxAttemptedAt   *time.Time
	MinAttemptedAt   *time.Time
	MinSegmentHealth float64
	MaxSegmentHealth float64
}

// Consumer defines the minimum number of methods to process segments from queue.
type Consumer interface {
	// Select gets an injured segments.
	Select(ctx context.Context, limit int, includedPlacements []storj.PlacementConstraint, excludedPlacements []storj.PlacementConstraint) ([]InjuredSegment, error)
	// Release releases an injured segment record. This should be called after
	// the segment is acquired by Select(), once the segment has been repaired
	// or the repair has failed.
	Release(ctx context.Context, s InjuredSegment, repaired bool) error
}

// RepairQueue implements queueing for segments that need repairing.
// Implementation can be found at satellite/satellitedb/repairqueue.go.
//
// architecture: Database
type RepairQueue interface {
	Consumer
	// Insert adds an injured segment.
	Insert(ctx context.Context, s *InjuredSegment) (alreadyInserted bool, err error)
	// InsertBatch adds multiple injured segments
	InsertBatch(ctx context.Context, segments []*InjuredSegment) (newlyInsertedSegments []*InjuredSegment, err error)
	// Delete removes an injured segment.
	Delete(ctx context.Context, s InjuredSegment) error
	// Clean removes all segments last updated before a certain time
	Clean(ctx context.Context, before time.Time) (deleted int64, err error)
	// SelectN lists limit amount of injured segments.
	SelectN(ctx context.Context, limit int) ([]InjuredSegment, error)
	// Count counts the number of segments in the repair queue.
	Count(ctx context.Context) (count int, err error)

	// Stat returns stat of the current queue state.
	Stat(ctx context.Context) ([]Stat, error)

	// TestingSetAttemptedTime sets attempted time for a segment.
	TestingSetAttemptedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error)
	// TestingSetUpdatedTime sets updated time for a segment. For testing only.
	TestingSetUpdatedTime(ctx context.Context, placement storj.PlacementConstraint, streamID uuid.UUID, position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error)
}
