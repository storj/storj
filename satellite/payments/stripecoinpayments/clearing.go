// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
)

// ErrClearing is stripecoinpayments clearing loop error class.
var ErrClearing = errs.Class("stripecoinpayments clearing error")

// Clearing runs process of reconciling transactions deposits,
// customer balance, invoices and usages.
type Clearing struct {
	log               *zap.Logger
	service           *Service
	TransactionCycle  sync2.Cycle
	ApplyBalanceCycle sync2.Cycle
}

// NewClearing creates new clearing loop.
func NewClearing(log *zap.Logger, service *Service, txInterval time.Duration) *Clearing {
	return &Clearing{
		log:              log,
		service:          service,
		TransactionCycle: *sync2.NewCycle(txInterval),
	}
}

// Run runs all clearing related cycles.
func (clearing *Clearing) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group

	clearing.TransactionCycle.Start(ctx, &group,
		func(ctx context.Context) error {
			clearing.log.Info("running transactions update cycle")

			if err := clearing.service.updateTransactionsLoop(ctx); err != nil {
				clearing.log.Error("transaction update cycle failed", zap.Error(ErrClearing.Wrap(err)))
			}

			return nil
		},
	)
	clearing.ApplyBalanceCycle.Start(ctx, &group,
		func(ctx context.Context) error {
			clearing.log.Info("running apply account balance cycle")

			if err := clearing.service.applyAccountBalanceLoop(ctx); err != nil {
				clearing.log.Error("account apply balance cycle failed", zap.Error(ErrClearing.Wrap(err)))
			}

			return nil
		},
	)

	return ErrClearing.Wrap(group.Wait())
}

// Close closes all underlying resources.
func (clearing *Clearing) Close() (err error) {
	defer mon.Task()(nil)(&err)

	clearing.TransactionCycle.Close()
	return nil
}
