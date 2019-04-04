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

// Raw mirrors dbx.AccountingRaw, allowing us to use that struct without leaking dbx
type Raw struct {
	ID              int64
	NodeID          storj.NodeID
	IntervalEndTime time.Time
	DataTotal       float64
	DataType        int
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
	// SaveAtRestRaw records raw tallies of at-rest-data.
	SaveAtRestRaw(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error
	// GetRaw retrieves all raw tallies
	GetRaw(ctx context.Context) ([]*Raw, error)
	// GetRawSince retrieves all raw tallies since latestRollup
	GetRawSince(ctx context.Context, latestRollup time.Time) ([]*Raw, error)
	// GetStoragenodeBandwidthSince retrieves all storagenode_bandwidth_rollup entires since latestRollup
	GetStoragenodeBandwidthSince(ctx context.Context, latestRollup time.Time) ([]*StoragenodeBandwidthRollup, error)
	// SaveRollup records raw tallies of at rest data to the database
	SaveRollup(ctx context.Context, latestTally time.Time, stats RollupStats) error
	// SaveBucketTallies saves the latest bucket info
	SaveBucketTallies(ctx context.Context, intervalStart time.Time, bucketTallies map[string]*BucketTally) error
	// QueryPaymentInfo queries Overlay, Accounting Rollup on nodeID
	QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*CSVRow, error)
	// DeleteRawBefore deletes all raw tallies prior to some time
	DeleteRawBefore(ctx context.Context, latestRollup time.Time) error
	// CreateBucketStorageTally creates a record for BucketStorageTally in the accounting DB table
	CreateBucketStorageTally(ctx context.Context, tally BucketStorageTally) error
	// ProjectBandwidthTotal returns the sum of GET bandwidth usage for a projectID in the past time frame
	ProjectBandwidthTotal(ctx context.Context, bucketID []byte, from time.Time) (int64, error)
	// ProjectStorageTotals returns the current inline and remote storage usage for a projectID
	ProjectStorageTotals(ctx context.Context, projectID uuid.UUID) (int64, int64, error)
}
