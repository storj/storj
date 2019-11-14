// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/memory"
)

// RollupStats is a convenience alias
type RollupStats map[time.Time]map[storj.NodeID]*Rollup

// StoragenodeStorageTally mirrors dbx.StoragenodeStorageTally, allowing us to use that struct without leaking dbx
type StoragenodeStorageTally struct {
	ID              int64
	NodeID          storj.NodeID
	IntervalEndTime time.Time
	DataTotal       float64
}

// StoragenodeBandwidthRollup mirrors dbx.StoragenodeBandwidthRollup, allowing us to use the struct without leaking dbx
type StoragenodeBandwidthRollup struct {
	NodeID        storj.NodeID
	IntervalStart time.Time
	Action        uint
	Settled       uint64
}

// Rollup mirrors dbx.AccountingRollup, allowing us to use that struct without leaking dbx
type Rollup struct {
	ID             int64
	NodeID         storj.NodeID
	StartTime      time.Time
	PutTotal       int64
	GetTotal       int64
	GetAuditTotal  int64
	GetRepairTotal int64
	PutRepairTotal int64
	AtRestTotal    float64
}

// StorageNodeUsage is node at rest space usage over a period of time
type StorageNodeUsage struct {
	NodeID      storj.NodeID
	StorageUsed float64

	Timestamp time.Time
}

// StoragenodeAccounting stores information about bandwidth and storage usage for storage nodes
//
// architecture: Database
type StoragenodeAccounting interface {
	// SaveTallies records tallies of data at rest
	SaveTallies(ctx context.Context, latestTally time.Time, nodeData map[storj.NodeID]float64) error
	// GetTallies retrieves all tallies
	GetTallies(ctx context.Context) ([]*StoragenodeStorageTally, error)
	// GetTalliesSince retrieves all tallies since latestRollup
	GetTalliesSince(ctx context.Context, latestRollup time.Time) ([]*StoragenodeStorageTally, error)
	// GetBandwidthSince retrieves all bandwidth rollup entires since latestRollup
	GetBandwidthSince(ctx context.Context, latestRollup time.Time) ([]*StoragenodeBandwidthRollup, error)
	// SaveRollup records tally and bandwidth rollup aggregations to the database
	SaveRollup(ctx context.Context, latestTally time.Time, stats RollupStats) error
	// LastTimestamp records and returns the latest last tallied time.
	LastTimestamp(ctx context.Context, timestampType string) (time.Time, error)
	// QueryPaymentInfo queries Nodes and Accounting_Rollup on nodeID
	QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*CSVRow, error)
	// QueryStorageNodeUsage returns slice of StorageNodeUsage for given period
	QueryStorageNodeUsage(ctx context.Context, nodeID storj.NodeID, start time.Time, end time.Time) ([]StorageNodeUsage, error)
	// DeleteTalliesBefore deletes all tallies prior to some time
	DeleteTalliesBefore(ctx context.Context, latestRollup time.Time) error
}

// ProjectAccounting stores information about bandwidth and storage usage for projects
//
// architecture: Database
type ProjectAccounting interface {
	// SaveTallies saves the latest project info
	SaveTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[string]*BucketTally) error
	// GetTallies retrieves all tallies
	GetTallies(ctx context.Context) ([]BucketTally, error)
	// CreateStorageTally creates a record for BucketStorageTally in the accounting DB table
	CreateStorageTally(ctx context.Context, tally BucketStorageTally) error
	// GetAllocatedBandwidthTotal returns the sum of GET bandwidth usage allocated for a projectID in the past time frame
	GetAllocatedBandwidthTotal(ctx context.Context, projectID uuid.UUID, from time.Time) (int64, error)
	// GetStorageTotals returns the current inline and remote storage usage for a projectID
	GetStorageTotals(ctx context.Context, projectID uuid.UUID) (int64, int64, error)
	// GetProjectUsageLimits returns project usage limit
	GetProjectUsageLimits(ctx context.Context, projectID uuid.UUID) (memory.Size, error)
}

// Cache stores live information about project storage which has not yet been synced to ProjectAccounting.
//
// architecture: Database
type Cache interface {
	GetProjectStorageUsage(ctx context.Context, projectID uuid.UUID) (totalUsed int64, err error)
	AddProjectStorageUsage(ctx context.Context, projectID uuid.UUID, inlineSpaceUsed, remoteSpaceUsed int64) error
	ResetTotals(ctx context.Context) error
	Close() error
}
