// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

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

	started sync2.Fence
	root    context.Context

	mu         sync.Mutex
	done       bool
	satellites map[storj.NodeID]*sync2.Workplace
}

const (
	jobEmptyTrash   = 1
	jobRestoreTrash = 2
)

// NewTrashChore instantiates a new TrashChore. choreInterval is how often this
// chore runs, and trashExpiryInterval is passed into the EmptyTrash method to
// determine which trashed pieces should be deleted.
func NewTrashChore(log *zap.Logger, choreInterval, trashExpiryInterval time.Duration, trust *trust.Pool, store *Store) *TrashChore {
	return &TrashChore{
		log:                 log,
		trashExpiryInterval: trashExpiryInterval,
		store:               store,
		trust:               trust,

		Cycle:      sync2.NewCycle(choreInterval),
		satellites: map[storj.NodeID]*sync2.Workplace{},
	}
}

// Run starts the cycle.
func (chore *TrashChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if chore == nil {
		return nil
	}
	chore.root = ctx
	chore.started.Release()

	err = chore.Cycle.Run(ctx, func(ctx context.Context) error {
		chore.log.Debug("starting to empty trash")

		var wg sync.WaitGroup
		limiter := make(chan struct{}, 1)
		for _, satellite := range chore.trust.GetSatellites(ctx) {
			satellite := satellite
			place := chore.ensurePlace(satellite)
			wg.Add(1)
			ok := place.Start(chore.root, jobEmptyTrash, nil, func(ctx context.Context) {
				defer wg.Done()
				// don't allow multiple trash jobs at the same time
				select {
				case <-ctx.Done():
					return
				case limiter <- struct{}{}:
				}
				defer func() { <-limiter }()

				timeStart := time.Now()
				chore.log.Info("emptying trash started", zap.Stringer("Satellite ID", satellite))
				trashedBefore := time.Now().Add(-chore.trashExpiryInterval)
				err := chore.store.EmptyTrash(ctx, satellite, trashedBefore)
				if err != nil {
					chore.log.Error("emptying trash failed", zap.Error(err))
				} else {
					chore.log.Info("emptying trash finished", zap.Stringer("Satellite ID", satellite), zap.Duration("elapsed", time.Since(timeStart)))
				}
			})
			if !ok {
				wg.Done()
			}
		}
		wg.Wait()
		return nil
	})

	chore.mu.Lock()
	chore.done = true
	chore.mu.Unlock()

	for _, place := range chore.satellites {
		place.Cancel()
	}
	for _, place := range chore.satellites {
		<-place.Done()
	}

	return err
}

// Close closes the chore.
func (chore *TrashChore) Close() error {
	if chore != nil {
		chore.Cycle.Close()
	}
	return nil
}

// StartRestore starts a satellite restore, if it hasn't already started and
// the chore is not shutting down.
func (chore *TrashChore) StartRestore(ctx context.Context, satellite storj.NodeID) error {
	if !chore.started.Wait(ctx) {
		return ctx.Err()
	}

	place := chore.ensurePlace(satellite)
	if place == nil {
		return context.Canceled
	}

	place.Start(chore.root, jobRestoreTrash, func(jobID interface{}) bool {
		return jobID == jobEmptyTrash
	}, func(ctx context.Context) {
		chore.log.Info("restore trash started", zap.Stringer("Satellite ID", satellite))
		err := chore.store.RestoreTrash(ctx, satellite)
		if err != nil {
			chore.log.Error("restore trash failed", zap.Stringer("Satellite ID", satellite), zap.Error(err))
		} else {
			chore.log.Info("restore trash finished", zap.Stringer("Satellite ID", satellite))
		}
	})

	return nil
}

// ensurePlace creates a work place for the specified satellite.
func (chore *TrashChore) ensurePlace(satellite storj.NodeID) *sync2.Workplace {
	chore.mu.Lock()
	defer chore.mu.Unlock()
	if chore.done {
		return nil
	}

	place, ok := chore.satellites[satellite]
	if !ok {
		place = sync2.NewWorkPlace()
		chore.satellites[satellite] = place
	}
	return place
}
