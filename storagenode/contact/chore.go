// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"math/rand"
	"strconv"
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

	interval time.Duration
	Cycles   []*sync2.Cycle
}

var (
	errPingSatellite = errs.Class("ping satellite error")
)

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

	for _, satellite := range chore.trust.GetSatellites(ctx) {
		satellite := satellite
		// set backOff interval to a random value [1, 6) to create some jitter
		rand.Seed(time.Now().UnixNano())
		backOff := time.Duration(rand.Int63n(int64(6*time.Second)) + 1)

		interval := chore.interval
		cycle := sync2.NewCycle(interval)
		chore.Cycles = append(chore.Cycles, cycle)

		cycle.Start(ctx, &group, func(ctx context.Context) error {
			err := chore.pingSatellite(ctx, satellite)
			if err == nil {
				return nil
			}

			interval := backOff
			retries := 0
			for {
				if !sync2.Sleep(ctx, interval) {
					chore.log.Error("ping satellite failed after " + strconv.Itoa(retries) + " retries timed out")
					return ctx.Err()
				}
				err := chore.pingSatellite(ctx, satellite)
				retries++
				if err == nil {
					return nil
				}
				chore.log.Error("ping satellite failed " + strconv.Itoa(retries) + " times")
				interval *= 2
				if interval > chore.interval {
					interval = chore.interval
				}
			}
		})
	}
	return group.Wait()
}

func (chore *Chore) pingSatellite(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	self := chore.service.Local()
	address, err := chore.trust.GetAddress(ctx, id)
	if err != nil {
		return errPingSatellite.New("failed to get satellite address %s: %v", id.String(), err)
	}
	conn, err := chore.dialer.DialAddressID(ctx, address, id)
	if err != nil {
		return errPingSatellite.New("failed to dial satellite address %s %s: %v", id.String(), address, err)
	}
	_, err = conn.NodeClient().CheckIn(ctx, &pb.CheckInRequest{
		Address:  self.Address.GetAddress(),
		Version:  &self.Version,
		Capacity: &self.Capacity,
		Operator: &self.Operator,
	})
	if err != nil {
		return errPingSatellite.New("failed to check into satellite %s: %v", id.String(), err)
	}
	return nil
}

// Pause pauses all the cycles in the contact chore
func (chore *Chore) Pause() {
	for _, loop := range chore.Cycles {
		loop.Pause()
	}
}

// TriggerWait ensures that each cycle is done at least once and waits for completion.
// If the cycle is currently running it waits for the previous to complete and then runs.
func (chore *Chore) TriggerWait() {
	for _, loop := range chore.Cycles {
		loop.TriggerWait()
	}
}

// Close stops all the cycles in the contact chore
func (chore *Chore) Close() error {
	for _, cycle := range chore.Cycles {
		cycle.Close()
	}
	return nil
}
