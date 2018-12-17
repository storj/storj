// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
)

//DB is an interface for interacting with accounting stuff
type DB interface {
	// LastBWGranularTime records the greatest last tallied bandwidth agreement time
	LastBWGranularTime(ctx context.Context) (time.Time, bool, error)
	// SaveBWRaw records raw sums of bw agreement values to the database
	// and updates the LastBWGranularTime
	SaveBWRaw(ctx context.Context, logger *zap.Logger, latestBwa time.Time, bwTotals map[string]int64) error
	// SaveAtRestRaw records raw tallies of at rest data to the database
	SaveAtRestRaw(ctx context.Context, logger *zap.Logger, nodeData map[storj.NodeID]int64) error
}
