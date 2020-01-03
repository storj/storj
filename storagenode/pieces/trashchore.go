// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/sync2"
	"storj.io/storj/storagenode/trust"
)

// TrashChore is the chore that periodically empties the trash
type TrashChore struct {
	log                 *zap.Logger
	interval            time.Duration
	trashExpiryInterval time.Duration
	store               *Store
	trust               *trust.Pool
	cycle               *sync2.Cycle
	started             sync2.Fence
}

// NewTrashChore instantiates a new TrashChore. choreInterval is how often this
// chore runs, and trashExpiryInterval is passed into the EmptyTrash method to
// determine which trashed pieces should be deleted
func NewTrashChore(log *zap.Logger, choreInterval, trashExpiryInterval time.Duration, trust *trust.Pool, store *Store) *TrashChore {
	return &TrashChore{
		log:                 log,
		interval:            choreInterval,
		trashExpiryInterval: trashExpiryInterval,
		store:               store,
		trust:               trust,
	}
}

// Run starts the cycle
func (chore *TrashChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	chore.log.Info("Storagenode TrashChore starting up")

	chore.cycle = sync2.NewCycle(chore.interval)
	chore.cycle.Start(ctx, &errgroup.Group{}, func(ctx context.Context) error {
		chore.log.Debug("starting EmptyTrash cycle")

		for _, satelliteID := range chore.trust.GetSatellites(ctx) {
			trashedBefore := time.Now().Add(-chore.trashExpiryInterval)
			err := chore.store.EmptyTrash(ctx, satelliteID, trashedBefore)
			if err != nil {
				chore.log.Error("EmptyTrash cycle failed", zap.Error(err))
			}
		}

		return nil
	})
	chore.started.Release()
	return err
}

// TriggerWait ensures that the cycle is done at least once and waits for
// completion.  If the cycle is currently running it waits for the previous to
// complete and then runs.
func (chore *TrashChore) TriggerWait(ctx context.Context) {
	chore.started.Wait(ctx)
	chore.cycle.TriggerWait()
}

// Close the chore
func (chore *TrashChore) Close() error {
	if chore.cycle != nil {
		chore.cycle.Close()
	}
	return nil
}
