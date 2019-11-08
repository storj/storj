// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reportedrollup

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/orders"
)

var (
	mon = monkit.Package()

	// Error is the error class for this package
	Error = errs.Class("reportedrollup")
)

// Config is a configuration struct for the Chore.
type Config struct {
	Interval time.Duration `help:"how often to flush the reported serial rollups to the database" devDefault:"5m" releaseDefault:"24h"`
}

// Chore for flushing reported serials to the database as rollups.
//
// architecture: Chore
type Chore struct {
	log  *zap.Logger
	db   orders.DB
	Loop *sync2.Cycle
}

// NewChore creates new chore for flushing the reported serials to the database as rollups.
func NewChore(log *zap.Logger, db orders.DB, config Config) *Chore {
	return &Chore{
		log:  log,
		db:   db,
		Loop: sync2.NewCycle(config.Interval),
	}
}

// Run starts the reported rollups chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		err := chore.RunOnce(ctx, time.Now())
		if err != nil {
			chore.log.Error("error flushing reported rollups", zap.Error(err))
		}
		return nil
	})
}

// Close stops the reported rollups chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// RunOnce finds expired bandwidth as of 'now' and inserts rollups into the appropriate tables.
func (chore *Chore) RunOnce(ctx context.Context, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	bucketRollups, storagenodeRollups, err := chore.db.GetBillableBandwidth(ctx, now)
	if err != nil {
		return err
	}

	return Error.Wrap(chore.db.WithTransaction(ctx, func(ctx context.Context, tx orders.Transaction) error {
		if err := tx.UpdateBucketBandwidthBatch(ctx, now, bucketRollups); err != nil {
			return Error.Wrap(err)
		}
		if err := tx.UpdateStoragenodeBandwidthBatch(ctx, now, storagenodeRollups); err != nil {
			return Error.Wrap(err)
		}
		if err := tx.DeleteExpiredReportedSerials(ctx, now); err != nil {
			return Error.Wrap(err)
		}
		return nil
	}))
}
