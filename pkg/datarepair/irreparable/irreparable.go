// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparable

import (
	"context"
)

// DB interface for database operations
type DB interface {
	// IncrementRepairAttempts increments the repair attempt
	IncrementRepairAttempts(ctx context.Context, segmentInfo *RemoteSegmentInfo) error
	// Get a irreparable's segment info from the db
	Get(ctx context.Context, segmentPath []byte) (*RemoteSegmentInfo, error)
	// Delete a irreparable's segment info from the db
	Delete(ctx context.Context, segmentPath []byte) error
}

// RemoteSegmentInfo is info about a single entry stored in the irreparable
type RemoteSegmentInfo struct {
	EncryptedSegmentPath   []byte
	EncryptedSegmentDetail []byte //contains marshaled info of pb.Pointer
	LostPiecesCount        int64
	RepairUnixSec          int64
	RepairAttemptCount     int64
}
