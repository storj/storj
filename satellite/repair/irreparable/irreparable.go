// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"

	"storj.io/common/pb"
)

// DB stores information about repairs that have failed.
//
// architecture: Database
type DB interface {
	// IncrementRepairAttempts increments the repair attempts.
	IncrementRepairAttempts(ctx context.Context, segmentInfo *pb.IrreparableSegment) error
	// Get returns irreparable segment info based on segmentPath.
	Get(ctx context.Context, segmentPath []byte) (*pb.IrreparableSegment, error)
	// GetLimited returns a list of irreparable segment info starting after the last segment info we retrieved
	GetLimited(ctx context.Context, limit int, lastSeenSegmentPath []byte) ([]*pb.IrreparableSegment, error)
	// Delete removes irreparable segment info based on segmentPath.
	Delete(ctx context.Context, segmentPath []byte) error
}
