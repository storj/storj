// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/sync2"
)

// Chore is the contact chore for nodes announcing themselves to their trusted satellites.
//
// architecture: Chore
type Chore struct {
	log     *zap.Logger
	service *Service

	mu      sync.Mutex
	cycles  map[storj.NodeID]*sync2.Cycle
	started sync2.Fence

	interval, timeout time.Duration
}

// NewChore creates a new contact chore.
func NewChore(log *zap.Logger, interval, timeout time.Duration, service *Service) *Chore {
	return &Chore{
		log:     log,
		service: service,

		cycles:   make(map[storj.NodeID]*sync2.Cycle),
		interval: interval,
		timeout:  timeout,
	}
}

// Run the contact chore on a regular interval with jitter.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	var group errgroup.Group

	if !chore.service.initialized.Wait(ctx) {
		return ctx.Err()
	}

	// configure the satellite ping cycles
	chore.updateCycles(ctx, &group, chore.service.trust.GetSatellites(ctx))

	// set up a cycle to update ping cycles on a frequent interval
	refreshCycle := sync2.NewCycle(time.Minute)
	refreshCycle.Start(ctx, &group, func(ctx context.Context) error {
		chore.updateCycles(ctx, &group, chore.service.trust.GetSatellites(ctx))
		return nil
	})

	defer refreshCycle.Close()

	chore.started.Release()
	return group.Wait()
}

func (chore *Chore) updateCycles(ctx context.Context, group *errgroup.Group, satellites []storj.NodeID) {
	chore.mu.Lock()
	defer chore.mu.Unlock()

	trustedIDs := make(map[storj.NodeID]struct{})

	for _, satellite := range satellites {
		satellite := satellite // alias the loop var since it is captured below

		trustedIDs[satellite] = struct{}{}
		if _, ok := chore.cycles[satellite]; ok {
			// Ping cycle has already been started for this satellite
			continue
		}

		// Set up a new ping cycle for the newly trusted satellite
		chore.log.Debug("Starting cycle", zap.Stringer("Satellite ID", satellite))
		cycle := sync2.NewCycle(chore.interval)
		chore.cycles[satellite] = cycle
		cycle.Start(ctx, group, func(ctx context.Context) error {
			return chore.service.pingSatellite(ctx, satellite, chore.interval, chore.timeout)
		})
	}

	// Stop the ping cycle for satellites that are no longer trusted
	for satellite, cycle := range chore.cycles {
		if _, ok := trustedIDs[satellite]; !ok {
			chore.log.Debug("Stopping cycle", zap.Stringer("Satellite ID", satellite))
			cycle.Close()
			delete(chore.cycles, satellite)
		}
	}
}

// Pause stops all the cycles in the contact chore.
func (chore *Chore) Pause(ctx context.Context) {
	chore.started.Wait(ctx)
	chore.mu.Lock()
	defer chore.mu.Unlock()
	for _, cycle := range chore.cycles {
		cycle.Pause()
	}
}

// Restart restarts all the cycles in the contact chore.
func (chore *Chore) Restart(ctx context.Context) {
	chore.started.Wait(ctx)
	chore.mu.Lock()
	defer chore.mu.Unlock()
	for _, cycle := range chore.cycles {
		cycle.Restart()
	}
}

// Trigger ensures that each cycle is done at least once.
// If the cycle is currently running it waits for the previous to complete and then runs.
func (chore *Chore) Trigger(ctx context.Context) {
	chore.started.Wait(ctx)
	chore.mu.Lock()
	defer chore.mu.Unlock()
	for _, cycle := range chore.cycles {
		cycle := cycle
		go func() {
			cycle.Trigger()
		}()
	}
}

// TriggerWait ensures that each cycle is done at least once and waits for completion.
// If the cycle is currently running it waits for the previous to complete and then runs.
func (chore *Chore) TriggerWait(ctx context.Context) {
	chore.started.Wait(ctx)
	chore.mu.Lock()
	defer chore.mu.Unlock()
	var group errgroup.Group
	for _, cycle := range chore.cycles {
		cycle := cycle
		group.Go(func() error {
			cycle.TriggerWait()
			return nil
		})
	}
	_ = group.Wait() // goroutines aren't returning any errors
}

// Close stops all the cycles in the contact chore.
func (chore *Chore) Close() error {
	chore.mu.Lock()
	defer chore.mu.Unlock()
	for _, cycle := range chore.cycles {
		cycle.Close()
	}
	chore.cycles = nil
	return nil
}
