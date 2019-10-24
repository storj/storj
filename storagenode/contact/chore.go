// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"math/rand"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

// Chore is the contact chore for nodes announcing themselves to their trusted satellites
//
// architecture: Chore
type Chore struct {
	log     *zap.Logger
	service *Service
	dialer  rpc.Dialer

	satelliteURL storj.NodeURL

	backOff  time.Duration
	maxDelay time.Duration
	Loop     *sync2.Cycle
}

var (
	errContactChore = errs.Class("contact chore error")
)

// NewChore creates a new contact chore
func NewChore(log *zap.Logger, satelliteURL storj.NodeURL, interval time.Duration, dialer rpc.Dialer, service *Service) *Chore {
	return &Chore{
		log:     log,
		service: service,
		dialer:  dialer,

		satelliteURL: satelliteURL,

		backOff:  time.Duration(rand.Int63n(int64(5 * time.Second))),
		maxDelay: interval,
		Loop:     sync2.NewCycle(interval),
	}
}

// Run the contact chore on a regular interval with jitter
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	chore.log.Info("Storagenode contact chore starting up")

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		if err := chore.pingSatelliteWithBackOff(ctx); err != nil {
			chore.log.Error("pingSatelliteWithBackOff failed", zap.Error(err))
		}
		return nil
	})
}

func (chore *Chore) pingSatelliteWithBackOff(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	for {
		err = chore.pingSatellite(ctx)
		if err != nil {
			chore.log.Info("pingSatellite failed", zap.Error(err))
			if ticking := sync2.Sleep(ctx, chore.maxDelay); ticking == false {
				chore.log.Info("pingSatellite exiting before successful connection")
				return nil
			}
			chore.backOff *= 2
			if chore.backOff > chore.maxDelay {
				chore.backOff = chore.maxDelay
			}
			time.Sleep(chore.backOff)
			chore.log.Info("reattempt pingSatellite")
			continue
		}
		return nil
	}
}

func (chore *Chore) pingSatellite(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	self := chore.service.Local()
	conn, err := chore.dialer.DialAddressID(ctx, chore.satelliteURL.Address, chore.satelliteURL.ID)
	if err != nil {
		return errContactChore.New("failed creating connection to satellite %v", chore.satelliteURL.ID.String())
	}
	_, err = conn.NodeClient().CheckIn(ctx, &pb.CheckInRequest{
		Address:  self.Address.GetAddress(),
		Version:  &self.Version,
		Capacity: &self.Capacity,
		Operator: &self.Operator,
	})
	if err != nil {
		return errContactChore.New("failed checkin with satellite %v", chore.satelliteURL.ID.String())
	}
	return nil
}

// Close stops the contact chore
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
