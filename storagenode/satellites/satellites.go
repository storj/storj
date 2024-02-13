// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellites

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// Status refers to the state of the relationship with a satellites.
type Status = int

// It is important that the values/order of these Status constants are not changed
// because they are stored in the database.
const (
	// Unexpected status should not be used for sanity checking.
	Unexpected Status = 0
	// Normal status reflects a lack of graceful exit.
	Normal Status = 1
	// Exiting reflects an active graceful exit.
	Exiting Status = 2
	// ExitSucceeded reflects a graceful exit that succeeded.
	ExitSucceeded Status = 3
	// ExitFailed reflects a graceful exit that failed.
	ExitFailed Status = 4
	// Untrusted reflects a satellite that is not trusted.
	Untrusted Status = 5
	// CleanupInProgress reflects a satellite that is being cleaned up.
	CleanupInProgress Status = 6
	// CleanupFailed reflects a satellite that failed to be cleaned up.
	CleanupFailed Status = 7
	// CleanupSucceeded reflects a satellite that was successfully cleaned up.
	CleanupSucceeded Status = 8
)

// ExitProgress contains the status of a graceful exit.
type ExitProgress struct {
	SatelliteID       storj.NodeID
	InitiatedAt       *time.Time
	FinishedAt        *time.Time
	StartingDiskUsage int64
	BytesDeleted      int64
	CompletionReceipt []byte
	Status            Status
}

// Satellite contains the satellite and status.
type Satellite struct {
	SatelliteID storj.NodeID
	Address     string
	AddedAt     time.Time
	Status      Status
}

// DB works with satellite database.
//
// architecture: Database
type DB interface {
	// SetAddress inserts into satellite's db id, address.
	SetAddress(ctx context.Context, satelliteID storj.NodeID, address string) error
	// SetAddressAndStatus inserts into satellite's db id, address and status.
	SetAddressAndStatus(ctx context.Context, satelliteID storj.NodeID, address string, status Status) error
	// GetSatellite retrieves that satellite by ID
	GetSatellite(ctx context.Context, satelliteID storj.NodeID) (satellite Satellite, err error)
	// GetSatellites retrieves all satellites, including untrusted ones.
	GetSatellites(ctx context.Context) (sats []Satellite, err error)
	// DeleteSatellite removes that satellite by ID.
	DeleteSatellite(ctx context.Context, satelliteID storj.NodeID) error
	// UpdateSatelliteStatus updates the status of the satellite.
	UpdateSatelliteStatus(ctx context.Context, satelliteID storj.NodeID, status Status) error
	// GetSatellitesUrls retrieves all satellite's id and urls.
	GetSatellitesUrls(ctx context.Context) (satelliteURLs []storj.NodeURL, err error)
	// InitiateGracefulExit updates the database to reflect the beginning of a graceful exit
	InitiateGracefulExit(ctx context.Context, satelliteID storj.NodeID, intitiatedAt time.Time, startingDiskUsage int64) error
	// CancelGracefulExit removes that satellite by ID
	CancelGracefulExit(ctx context.Context, satelliteID storj.NodeID) error
	// UpdateGracefulExit increments the total bytes deleted during a graceful exit
	UpdateGracefulExit(ctx context.Context, satelliteID storj.NodeID, bytesDeleted int64) error
	// CompleteGracefulExit updates the database when a graceful exit is completed or failed
	CompleteGracefulExit(ctx context.Context, satelliteID storj.NodeID, finishedAt time.Time, exitStatus Status, completionReceipt []byte) error
	// ListGracefulExits lists all graceful exit records
	ListGracefulExits(ctx context.Context) ([]ExitProgress, error)
}
