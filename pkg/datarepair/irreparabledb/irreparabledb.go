// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparabledb

import (
	"context"

	"github.com/zeebo/errs"
)

// Error is the default irreparabledb errs class
var Error = errs.Class("irreparabledb error")

// IrreparableDB interface for database operations
type IrreparableDB interface {
	// IncrementRepairAttempts increments the repair attempt
	IncrementRepairAttempts(context.Context, *RemoteSegmentInfo) error
	// Get a irreparable's segment info from the db
	Get(context.Context, []byte) (*RemoteSegmentInfo, error)
	// Delete a irreparable's segment info from the db
	Delete(ctx context.Context, segmentPath []byte) error
}

// Database implements the irreparabledb services
type Database struct {
	db IrreparableDB
}

// RemoteSegmentInfo is info about a single entry stored in the irreparable db
type RemoteSegmentInfo struct {
	EncryptedSegmentPath   []byte
	EncryptedSegmentDetail []byte //contains marshaled info of pb.Pointer
	LostPiecesCount        int64
	RepairUnixSec          int64
	RepairAttemptCount     int64
}
