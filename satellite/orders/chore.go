// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// Chore for flushing orders write cache to the database.
//
// architecture: Chore
type Chore struct {
	log              *zap.Logger
	ordersWriteCache *RollupsWriteCache
	Loop             *sync2.Cycle
}

// NewChore creates new chore for flushing the orders write cache to the database.
func NewChore(log *zap.Logger, ordersWriteCache *RollupsWriteCache, config Config) *Chore {
	return &Chore{
		log:              log,
		ordersWriteCache: ordersWriteCache,
		Loop:             sync2.NewCycle(config.FlushInterval),
	}
}

// Run starts the orders write cache chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		chore.ordersWriteCache.FlushToDB(ctx)
		return nil
	})
}

// Close stops the orders write cache chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
