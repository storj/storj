// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

//RollupStats is a convenience alias
type RollupStats map[time.Time]map[storj.NodeID]*Rollup

//Raw mirrors dbx.AccountingRaw, allowing us to use that struct without leaking dbx
type Raw struct {
	ID              int64
	NodeID          storj.NodeID
	IntervalEndTime time.Time
	DataTotal       float64
	DataType        int
	CreatedAt       time.Time
}

//Rollup mirrors dbx.AccountingRollup, allowing us to use that struct without leaking dbx
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

// DB stores information about bandwidth usage
type DB interface {
	// LastTimestamp records the latest last tallied time.
	LastTimestamp(ctx context.Context, timestampType string) (time.Time, error)
	// SaveBWRaw records raw sums of agreement values to the database and updates the LastTimestamp.
	SaveBWRaw(ctx context.Context, tallyEnd time.Time, created time.Time, bwTotals map[storj.NodeID][]int64) error
	// SaveAtRestRaw records raw tallies of at-rest-data.
	SaveAtRestRaw(ctx context.Context, latestTally time.Time, created time.Time, nodeData map[storj.NodeID]float64) error
	// GetRaw retrieves all raw tallies
	GetRaw(ctx context.Context) ([]*Raw, error)
	// GetRawSince r retrieves all raw tallies sinces
	GetRawSince(ctx context.Context, latestRollup time.Time) ([]*Raw, error)
	// SaveRollup records raw tallies of at rest data to the database
	SaveRollup(ctx context.Context, latestTally time.Time, stats RollupStats) error
	// QueryPaymentInfo queries StatDB, Accounting Rollup on nodeID
	QueryPaymentInfo(ctx context.Context, start time.Time, end time.Time) ([]*CSVRow, error)
}
