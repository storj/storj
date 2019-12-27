// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbcleanup

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

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
	SerialsInterval time.Duration `help:"how often to delete expired serial numbers" default:"24h"`
}

// Chore for deleting DB entries that are no longer needed.
//
// architecture: Chore
type Chore struct {
	log    *zap.Logger
	orders orders.DB

	Serials sync2.Cycle
}

// NewChore creates new chore for deleting DB entries.
func NewChore(log *zap.Logger, orders orders.DB, config Config) *Chore {
	return &Chore{
		log:    log,
		orders: orders,

		Serials: *sync2.NewCycle(config.SerialsInterval),
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

	deleted, err := chore.orders.DeleteExpiredSerials(ctx, time.Now().UTC())
	if err != nil {
		chore.log.Error("deleting expired serial numbers", zap.Error(err))
		return nil
	}

	chore.log.Debug("expired serials deleted", zap.Int("items deleted", deleted))
	return nil
}

// Close stops the dbcleanup chore.
func (chore *Chore) Close() error {
	chore.Serials.Close()
	return nil
}
