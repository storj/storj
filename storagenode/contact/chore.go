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

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
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
	cycles   []*sync2.Cycle
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

	chore.mu.Lock()
	for _, satellite := range chore.trust.GetSatellites(ctx) {
		satellite := satellite

		cycle := sync2.NewCycle(chore.interval)
		chore.cycles = append(chore.cycles, cycle)

		cycle.Start(ctx, &group, func(ctx context.Context) error {
			chore.log.Debug("starting cycle", zap.Stringer("Satellite ID", satellite))
			interval := initialBackOff
			attempts := 0
			for {
				err := chore.pingSatellite(ctx, satellite)
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
		})
	}
	chore.mu.Unlock()
	chore.started.Release()
	return group.Wait()
}

func (chore *Chore) pingSatellite(ctx context.Context, id storj.NodeID) (err error) {
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
	_, err = conn.NodeClient().CheckIn(ctx, &pb.CheckInRequest{
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
