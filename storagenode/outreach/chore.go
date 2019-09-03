// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package outreach

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()
)

// Config contains configurable parameters for outreach chore
// TODO: Hide MaxSleep from CLI
type Config struct {
	Interval time.Duration `help:"how frequently the node outreach chore should run" releaseDefault:"1h" devDefault:"30s"`
	MaxSleep time.Duration `help:"maximum duration to wait before pinging satellites" releaseDefault:"45m" devDefault:"0s"`
}

// Chore is the outreach chore for nodes announcing themselves to their trusted satellites
type Chore struct {
	log   *zap.Logger
	trust *trust.Pool

	maxSleep time.Duration
	Loop     *sync2.Cycle
}

// NewChore creates a new outreach chore
func NewChore(log *zap.Logger, interval time.Duration, maxSleep time.Duration, trust *trust.Pool) *Chore {
	return &Chore{
		log:   log,
		trust: trust,

		maxSleep: maxSleep,
		Loop:     sync2.NewCycle(interval),
	}
}

// Run the outreach chore on a regular interval with jitter
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	chore.log.Info("Storagenode outreach chore starting up")

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		if err := chore.sleep(ctx); err != nil {
			return err
		}
		if err = chore.pingSatellites(ctx); err != nil {
			chore.log.Error("pingSatellites failed", zap.Error(err))
		}
		return nil
	})
}

func (chore *Chore) pingSatellites(ctx context.Context) error {
	// loop through the trusted satellites
	// don't error out if an individual satellite ping fails
	// call awaitPingback helper method
	return nil
}

func awaitPingback() {
	// TODO: write a method that listens for each satellite to ping back, or it times out and receives a log message
	//  with which satellite they failed
	//  make sure to close the connections regardless
}

// sleep for random interval in [0;maxSleep)
// returns error if context was cancelled
func (chore *Chore) sleep(ctx context.Context) error {
	jitter := time.Duration(rand.Int63n(int64(chore.maxSleep)))
	if !sync2.Sleep(ctx, jitter) {
		return ctx.Err()
	}

	return nil
}
