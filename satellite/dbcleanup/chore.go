// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbcleanup

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
	// Error the default dbcleanup errs class.
	Error = errs.Class("dbcleanup error")

	mon = monkit.Package()
)

// Config defines configuration struct for dbcleanup chore.
type Config struct {
	SerialsInterval time.Duration `help:"how often to delete expired serial numbers" default:"4h"`
	BatchSize       int           `help:"number of records to delete per delete execution. 0 indicates no limit. applicable to cockroach DB only." default:"1000"`
}

// Chore for deleting DB entries that are no longer needed.
//
// architecture: Chore
type Chore struct {
	log    *zap.Logger
	orders orders.DB

	Serials *sync2.Cycle
	config  Config
	options *orders.SerialDeleteOptions
}

// NewChore creates new chore for deleting DB entries.
func NewChore(log *zap.Logger, ordersDB orders.DB, config Config) *Chore {
	var options *orders.SerialDeleteOptions
	if config.BatchSize > 0 {
		options = &orders.SerialDeleteOptions{
			BatchSize: config.BatchSize,
		}
	}

	return &Chore{
		log:    log,
		orders: ordersDB,

		Serials: sync2.NewCycle(config.SerialsInterval),
		config:  config,
		options: options,
	}
}

// Run starts the db cleanup chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Serials.Run(ctx, chore.deleteExpiredSerials)
}

func (chore *Chore) deleteExpiredSerials(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	chore.log.Debug("deleting expired serial numbers")

	now := time.Now()

	deleted, err := chore.orders.DeleteExpiredSerials(ctx, now, chore.options)
	if err != nil {
		chore.log.Error("deleting expired serial numbers", zap.Error(err))
	} else {
		chore.log.Debug("expired serials deleted", zap.Int("items deleted", deleted))
	}

	deleted, err = chore.orders.DeleteExpiredConsumedSerials(ctx, now)
	if err != nil {
		chore.log.Error("deleting expired serial numbers", zap.Error(err))
	} else {
		chore.log.Debug("expired serials deleted", zap.Int("items deleted", deleted))
	}

	return nil
}

// Close stops the dbcleanup chore.
func (chore *Chore) Close() error {
	chore.Serials.Close()
	return nil
}
