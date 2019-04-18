// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/storj"
)

// RollupStats is a convenience alias
type RollupStats map[time.Time]map[storj.NodeID]*Rollup

// StoragenodeStorageTally mirrors dbx.StoragenodeStorageTally allowing us to use that struct without leaking dbx
type StoragenodeStorageTally struct {
	ID              int64
	NodeID          storj.NodeID
	IntervalEndTime time.Time
	DataTotal       float64
	CreatedAt       time.Time
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

// DB stores information about bandwidth and storage usage
type DB interface {
	// LastTimestamp records the latest last tallied time.
	LastTimestamp(ctx context.Context, timestampType string) (time.Time, error)
	// SaveStoragenodeStorageTallies records the storagenode at rest data and updates LastTimestamp
	SaveStoragenodeStorageTallies(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error
	// GetStoragenodeStorage retrieves all the storagenode at rest data tallies
	GetStoragenodeStorage(ctx context.Context) ([]*StoragenodeStorageTally, error)
	// GetStoragenodeStorageSince retrieves all the storagenode at rest data tallies since latestRollup
	GetStoragenodeStorageSince(ctx context.Context, latestRollup time.Time) ([]*StoragenodeStorageTally, error)
	// GetStoragenodeBandwidthSince retrieves all storagenode_bandwidth_rollup entires since latestRollup
	GetStoragenodeBandwidthSince(ctx context.Context, latestRollup time.Time) ([]*StoragenodeBandwidthRollup, error)
	// SaveRollup records at rest tallies and bw rollups to the accounting_rollups table
	SaveRollup(ctx context.Context, latestTally time.Time, stats RollupStats) error
	// SaveBucketTallies saves the latest bucket info
	SaveBucketTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[string]*BucketTally) ([]BucketTally, error)
	// QueryPaymentInfo queries Overlay, Accounting Rollup on nodeID
	QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*CSVRow, error)
	// DeleteTalliesBefore deletes all storagenode storage tallies prior to some time
	DeleteTalliesBefore(ctx context.Context, latestRollup time.Time) error
	// CreateBucketStorageTally creates a record for BucketStorageTally in the accounting DB table
	CreateBucketStorageTally(ctx context.Context, tally BucketStorageTally) error
	// ProjectAllocatedBandwidthTotal returns the sum of GET bandwidth usage allocated for a projectID in the past time frame
	ProjectAllocatedBandwidthTotal(ctx context.Context, bucketID []byte, from time.Time) (int64, error)
	// ProjectStorageTotals returns the current inline and remote storage usage for a projectID
	ProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (int64, int64, error)
}
