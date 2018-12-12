// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Error is the default accountingdb errs class
var Error = errs.Class("accountingdb")

//DB is an interface for interacting with accounting stuff
type DB interface {
	LastGranularTime(ctx context.Context) (time.Time, bool, error)
	SaveGranulars(ctx context.Context, logger *zap.Logger, latestBwa time.Time, bwTotals map[string]int64) error
}
