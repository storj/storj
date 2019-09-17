// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellites

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

//Status refers to the state of the relationship with a satellites
type Status = int

const (
	//Unexpected status should not be used for sanity checking
	Unexpected Status = iota
	//Normal status reflects a lack of graceful exit
	Normal
	//Exiting reflects an active graceful exit
	Exiting
	//ExitedOk reflects a graceful exit that succeeded
	ExitedOk
	//ExitedFailed reflects a graceful exit that failed
	ExitedFailed
)

// DB works with satellite database
//
// architecture: Database
type DB interface {
	// initiate graceful exit
	InitiateGracefulExit(ctx context.Context, satelliteID storj.NodeID, intitiatedAt time.Time, startingDiskUsage int64) error
	// increment graceful exit bytes deleted
	UpdateGracefulExit(ctx context.Context, satelliteID storj.NodeID, bytesDeleted int64) error
	// complete graceful exit
	CompleteGracefulExit(ctx context.Context, satelliteID storj.NodeID, finishedAt time.Time, exitStatus Status, completionReceipt []byte) error
}
