// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"
)

// DB stores information about repairs that have failed.
type DB interface {
	// IncrementRepairAttempts increments the repair attempts.
	IncrementRepairAttempts(ctx context.Context, segmentInfo *RemoteSegmentInfo) error
	// Get returns irreparable segment info based on segmentPath.
	Get(ctx context.Context, segmentPath []byte) (*RemoteSegmentInfo, error)
	// Delete removes irreparable segment info based on segmentPath.
	Delete(ctx context.Context, segmentPath []byte) error
}

// RemoteSegmentInfo is information about failed repairs.
type RemoteSegmentInfo struct {
	EncryptedSegmentPath   []byte
	EncryptedSegmentDetail []byte //contains marshaled info of pb.Pointer
	LostPiecesCount        int64
	RepairUnixSec          int64
	RepairAttemptCount     int64
}
