// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"storj.io/storj/pkg/storj"
)

// DB stores information about bandwidth usage
type DB interface {
	// LastRawTime records the latest last tallied time.
	LastRawTime(ctx context.Context, timestampType string) (time.Time, bool, error)
	// SaveBWRaw records raw sums of agreement values to the database and updates the LastRawTime.
	SaveBWRaw(ctx context.Context, latestBwa time.Time, bwTotals map[string]int64) error
	// SaveAtRestRaw records raw tallies of at-rest-data.
	SaveAtRestRaw(ctx context.Context, latestTally time.Time, nodeData map[storj.NodeID]int64) error
}
