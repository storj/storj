// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/trust"
)

// TrashChore is the chore that periodically empties the trash.
type TrashChore struct {
	log                 *zap.Logger
	trashExpiryInterval time.Duration
	store               *Store
	trust               *trust.Pool

	Cycle *sync2.Cycle

	workers   workersService
	mu        sync.Mutex
	restoring map[storj.NodeID]bool
}

// NewTrashChore instantiates a new TrashChore. choreInterval is how often this
// chore runs, and trashExpiryInterval is passed into the EmptyTrash method to
// determine which trashed pieces should be deleted.
func NewTrashChore(log *zap.Logger, choreInterval, trashExpiryInterval time.Duration, trust *trust.Pool, store *Store) *TrashChore {
	return &TrashChore{
		log:                 log,
		trashExpiryInterval: trashExpiryInterval,
		store:               store,
		trust:               trust,

		Cycle:     sync2.NewCycle(choreInterval),
		restoring: map[storj.NodeID]bool{},
	}
}

// Run starts the cycle.
func (chore *TrashChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group
	chore.Cycle.Start(ctx, &group, func(ctx context.Context) error {
		chore.log.Debug("starting to empty trash")

		for _, satelliteID := range chore.trust.GetSatellites(ctx) {
			// ignore satellites that are being restored
			chore.mu.Lock()
			isRestoring := chore.restoring[satelliteID]
			chore.mu.Unlock()
			if isRestoring {
				continue
			}

			trashedBefore := time.Now().Add(-chore.trashExpiryInterval)
			err := chore.store.EmptyTrash(ctx, satelliteID, trashedBefore)
			if err != nil {
				chore.log.Error("emptying trash failed", zap.Error(err))
			}
		}

		return nil
	})
	group.Go(func() error {
		chore.workers.Run(ctx)
		return nil
	})
	return group.Wait()
}

// StartRestore starts restoring trash for the specified satellite.
func (chore *TrashChore) StartRestore(ctx context.Context, satellite storj.NodeID) {
	chore.mu.Lock()
	isRestoring := chore.restoring[satellite]
	if isRestoring {
		chore.mu.Unlock()
		return
	}
	chore.restoring[satellite] = true
	chore.mu.Unlock()

	ok := chore.workers.Go(ctx, func(ctx context.Context) {
		chore.log.Info("restore trash started", zap.Stringer("Satellite ID", satellite))
		err := chore.store.RestoreTrash(ctx, satellite)
		if err != nil {
			chore.log.Error("restore trash failed", zap.Stringer("Satellite ID", satellite), zap.Error(err))
		} else {
			chore.log.Info("restore trash finished", zap.Stringer("Satellite ID", satellite))
		}

		chore.mu.Lock()
		delete(chore.restoring, satellite)
		chore.mu.Unlock()
	})
	if !ok {
		chore.log.Info("failed to start restore trash", zap.Stringer("Satellite ID", satellite))
	}
}

// Close the chore.
func (chore *TrashChore) Close() error {
	chore.Cycle.Close()
	return nil
}

// workersService allows to start workers with a different context.
type workersService struct {
	started sync2.Fence
	root    context.Context
	active  sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

// Run starts waiting for worker requests with the specified context.
func (workers *workersService) Run(ctx context.Context) {
	// setup root context that the workers are bound to
	workers.root = ctx
	workers.started.Release()

	// wait until it's time to shut down:
	<-workers.root.Done()

	// ensure we don't allow starting workers after it's time to shut down
	workers.mu.Lock()
	workers.closed = true
	workers.mu.Unlock()

	// wait for any remaining workers
	workers.active.Wait()
}

// Go tries to start a worker.
func (workers *workersService) Go(ctx context.Context, work func(context.Context)) bool {
	// Wait until we can use workers.root.
	if !workers.started.Wait(ctx) {
		return false
	}

	// check that we are still allowed to start new workers
	workers.mu.Lock()
	if workers.closed {
		workers.mu.Unlock()
		return false
	}
	workers.active.Add(1)
	workers.mu.Unlock()

	go func() {
		defer workers.active.Done()
		work(workers.root)
	}()

	return true
}
