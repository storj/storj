// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellites

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// Status refers to the state of the relationship with a satellites
type Status = int

const (
	//Unexpected status should not be used for sanity checking
	Unexpected Status = 0
	//Normal status reflects a lack of graceful exit
	Normal = 1
	//Exiting reflects an active graceful exit
	Exiting = 2
	//ExitSucceeded reflects a graceful exit that succeeded
	ExitSucceeded = 3
	//ExitFailed reflects a graceful exit that failed
	ExitFailed = 4
)

// ExitProgress contains the status of a graceful exit
type ExitProgress struct {
	SatelliteID       storj.NodeID
	InitiatedAt       *time.Time
	FinishedAt        *time.Time
	StartingDiskUsage int64
	BytesDeleted      int64
	CompletionReceipt []byte
}

// Satellite contains the satellite and status
type Satellite struct {
	SatelliteID storj.NodeID
	AddedAt     time.Time
	Status      int32
}

// DB works with satellite database
//
// architecture: Database
type DB interface {
	// GetSatellite retrieves that satellite by ID
	GetSatellite(ctx context.Context, satelliteID storj.NodeID) (satellite Satellite, err error)
	// InitiateGracefulExit updates the database to reflect the beginning of a graceful exit
	InitiateGracefulExit(ctx context.Context, satelliteID storj.NodeID, intitiatedAt time.Time, startingDiskUsage int64) error
	// UpdateGracefulExit increments the total bytes deleted during a graceful exit
	UpdateGracefulExit(ctx context.Context, satelliteID storj.NodeID, bytesDeleted int64) error
	// CompleteGracefulExit updates the database when a graceful exit is completed or failed
	CompleteGracefulExit(ctx context.Context, satelliteID storj.NodeID, finishedAt time.Time, exitStatus Status, completionReceipt []byte) error
	// ListGracefulExits lists all graceful exit records
	ListGracefulExits(ctx context.Context) ([]ExitProgress, error)
}
