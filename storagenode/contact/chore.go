// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/trust"
)

// Chore is the contact chore for nodes announcing themselves to their trusted satellites
//
// architecture: Chore
type Chore struct {
	log     *zap.Logger
	service *Service
	dialer  rpc.Dialer

	trust *trust.Pool

	mu       sync.Mutex
	cycles   map[storj.NodeID]*sync2.Cycle
	started  sync2.Fence
	interval time.Duration
}

var (
	errPingSatellite = errs.Class("ping satellite error")
)

const initialBackOff = time.Second

// NewChore creates a new contact chore
func NewChore(log *zap.Logger, interval time.Duration, trust *trust.Pool, dialer rpc.Dialer, service *Service) *Chore {
	return &Chore{
		log:     log,
		service: service,
		dialer:  dialer,

		trust: trust,

		cycles:   make(map[storj.NodeID]*sync2.Cycle),
		interval: interval,
	}
}

// Run the contact chore on a regular interval with jitter
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	chore.log.Info("Storagenode contact chore starting up")

	var group errgroup.Group

	if !chore.service.initialized.Wait(ctx) {
		return ctx.Err()
	}

	// configure the satellite ping cycles
	chore.updateCycles(ctx, &group, chore.trust.GetSatellites(ctx))

	// set up a cycle to update ping cycles on a frequent interval
	refreshCycle := sync2.NewCycle(time.Minute)
	refreshCycle.Start(ctx, &group, func(ctx context.Context) error {
		chore.updateCycles(ctx, &group, chore.trust.GetSatellites(ctx))
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
			return chore.pingSatellite(ctx, satellite)
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

func (chore *Chore) pingSatellite(ctx context.Context, satellite storj.NodeID) error {
	interval := initialBackOff
	attempts := 0
	for {

		mon.Meter("satellite_contact_request").Mark(1) //locked

		err := chore.pingSatelliteOnce(ctx, satellite)
		attempts++
		if err == nil {
			return nil
		}
		chore.log.Error("ping satellite failed ", zap.Stringer("Satellite ID", satellite), zap.Int("attempts", attempts), zap.Error(err))

		// Sleeps until interval times out, then continue. Returns if context is cancelled.
		if !sync2.Sleep(ctx, interval) {
			chore.log.Info("context cancelled", zap.Stringer("Satellite ID", satellite))
			return nil
		}
		interval *= 2
		if interval >= chore.interval {
			chore.log.Info("retries timed out for this cycle", zap.Stringer("Satellite ID", satellite))
			return nil
		}
	}

}

func (chore *Chore) pingSatelliteOnce(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx, id)(&err)

	self := chore.service.Local()
	address, err := chore.trust.GetAddress(ctx, id)
	if err != nil {
		return errPingSatellite.Wrap(err)
	}

	conn, err := chore.dialer.DialAddressID(ctx, address, id)
	if err != nil {
		return errPingSatellite.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	_, err = pb.NewDRPCNodeClient(conn.Raw()).CheckIn(ctx, &pb.CheckInRequest{
		Address:  self.Address.GetAddress(),
		Version:  &self.Version,
		Capacity: &self.Capacity,
		Operator: &self.Operator,
	})
	if err != nil {
		return errPingSatellite.Wrap(err)
	}
	return nil
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
