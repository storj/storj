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
	log                *zap.Logger
	stripecoinpayments *Service
	transactionCycle   sync2.Cycle
}

// NewClearing creates new clearing loop.
func NewClearing(log *zap.Logger, service *Service, txInterval time.Duration) *Clearing {
	return &Clearing{
		log:                log,
		stripecoinpayments: service,
		transactionCycle:   *sync2.NewCycle(txInterval),
	}
}

// Run runs all clearing related cycles.
func (c *Clearing) Run(ctx context.Context) error {
	var group errgroup.Group

	c.transactionCycle.Start(ctx, &group,
		func(ctx context.Context) error {
			c.log.Info("running transactions update cycle")

			if err := c.stripecoinpayments.updateTransactionsLoop(ctx); err != nil {
				c.log.Error("transaction update cycle failed", zap.Error(ErrClearing.Wrap(err)))
			}

			return nil
		},
	)

	return group.Wait()
}

// Close closes all underlying resources.
func (c *Clearing) Close() error {
	c.transactionCycle.Close()
	return nil
}
