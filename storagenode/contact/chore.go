// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"math/rand"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
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

	maxSleep time.Duration
	Loop     *sync2.Cycle
}

// NewChore creates a new contact chore
func NewChore(log *zap.Logger, interval time.Duration, maxSleep time.Duration, trust *trust.Pool, dialer rpc.Dialer, service *Service) *Chore {
	return &Chore{
		log:     log,
		service: service,
		dialer:  dialer,

		trust: trust,

		maxSleep: maxSleep,
		Loop:     sync2.NewCycle(interval),
	}
}

// Run the contact chore on a regular interval with jitter
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	chore.log.Info("Storagenode contact chore starting up")

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		if err := chore.randomDurationSleep(ctx); err != nil {
			return err
		}
		if err := chore.pingSatellites(ctx); err != nil {
			chore.log.Error("pingSatellites failed", zap.Error(err))
		}
		return nil
	})
}

func (chore *Chore) pingSatellites(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	var group errgroup.Group
	self := chore.service.Local()
	satellites := chore.trust.GetSatellites(ctx)
	for _, satellite := range satellites {
		satellite := satellite
		addr, err := chore.trust.GetAddress(ctx, satellite)
		if err != nil {
			chore.log.Error("getting satellite address", zap.Error(err))
			continue
		}
		group.Go(func() error {
			conn, err := chore.dialer.DialAddressID(ctx, addr, satellite)
			if err != nil {
				return err
			}
			defer func() { err = errs.Combine(err, conn.Close()) }()

			_, err = conn.NodeClient().CheckIn(ctx, &pb.CheckInRequest{
				Address:  self.Address.GetAddress(),
				Version:  &self.Version,
				Capacity: &self.Capacity,
				Operator: &self.Operator,
			})

			return err
		})
	}

	return group.Wait()
}

// randomDurationSleep sleeps for random interval in [0;maxSleep)
// returns error if context was cancelled
func (chore *Chore) randomDurationSleep(ctx context.Context) error {
	if chore.maxSleep <= 0 {
		return nil
	}
	jitter := time.Duration(rand.Int63n(int64(chore.maxSleep)))
	if !sync2.Sleep(ctx, jitter) {
		return ctx.Err()
	}

	return nil
}

// Close stops the contact chore
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
