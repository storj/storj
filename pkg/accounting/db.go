// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"go.uber.org/zap"
)

//DB is an interface for interacting with accounting stuff
type DB interface {
	// LastGranularTime records the greatest last tallied bandwidth agreement time
	LastGranularTime(ctx context.Context) (time.Time, bool, error)
	// SaveGranulars records granular tallies (sums of bw agreement values) to the database
	// and updates the LastGranularTime
	SaveGranulars(ctx context.Context, logger *zap.Logger, latestBwa time.Time, bwTotals map[string]int64) error
}
