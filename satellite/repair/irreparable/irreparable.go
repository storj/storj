// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"

	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metainfo/metabase"
)

// DB stores information about repairs that have failed.
//
// architecture: Database
type DB interface {
	// IncrementRepairAttempts increments the repair attempts.
	IncrementRepairAttempts(ctx context.Context, segmentInfo *internalpb.IrreparableSegment) error
	// Get returns irreparable segment info based on segmentKey.
	Get(ctx context.Context, segmentKey metabase.SegmentKey) (*internalpb.IrreparableSegment, error)
	// GetLimited returns a list of irreparable segment info starting after the last segment info we retrieved
	GetLimited(ctx context.Context, limit int, lastSeenSegmentKey metabase.SegmentKey) ([]*internalpb.IrreparableSegment, error)
	// Delete removes irreparable segment info based on segmentKey.
	Delete(ctx context.Context, segmentKey metabase.SegmentKey) error
}
