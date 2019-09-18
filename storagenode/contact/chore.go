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
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode/trust"
)

var mon = monkit.Package()

// Config contains configurable parameters for contact chore
type Config struct {
	Interval time.Duration `help:"how frequently the node contact chore should run" releaseDefault:"1h" devDefault:"30s"`
	// MaxSleep should remain at default value to decrease traffic congestion to satellite
	MaxSleep time.Duration `help:"maximum duration to wait before pinging satellites" releaseDefault:"45m" devDefault:"0s" hidden:"true"`
}

// Chore is the contact chore for nodes announcing themselves to their trusted satellites
//
// architecture: Chore
type Chore struct {
	log       *zap.Logger
	rt        *kademlia.RoutingTable
	transport transport.Client

	trust *trust.Pool

	maxSleep time.Duration
	Loop     *sync2.Cycle
}

// NewChore creates a new contact chore
func NewChore(log *zap.Logger, interval time.Duration, maxSleep time.Duration, trust *trust.Pool, transport transport.Client, rt *kademlia.RoutingTable) *Chore {
	return &Chore{
		log:       log,
		rt:        rt,
		transport: transport,

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
	self := chore.rt.Local()
	satellites := chore.trust.GetSatellites(ctx)
	for _, satellite := range satellites {
		satellite := satellite
		addr, err := chore.trust.GetAddress(ctx, satellite)
		if err != nil {
			chore.log.Error("getting satellite address", zap.Error(err))
			continue
		}
		group.Go(func() error {
			conn, err := chore.transport.DialNode(ctx, &pb.Node{
				Id: satellite,
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   addr,
				},
			})
			if err != nil {
				return err
			}
			defer func() {
				if cerr := conn.Close(); cerr != nil {
					err = errs.Combine(err, cerr)
				}
			}()
			_, err = pb.NewNodeClient(conn).CheckIn(ctx, &pb.CheckInRequest{
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
