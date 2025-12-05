// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/sync2"
)

// Chore checks for satellites that the node is exiting and creates a worker per satellite to complete the process.
//
// architecture: Chore
type Chore struct {
	log    *zap.Logger
	dialer rpc.Dialer
	config Config

	service *Service

	exitingMap sync.Map
	Loop       *sync2.Cycle
	limiter    *sync2.Limiter
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, service *Service, dialer rpc.Dialer, config Config) *Chore {
	return &Chore{
		log:     log,
		dialer:  dialer,
		service: service,
		config:  config,
		Loop:    sync2.NewCycle(config.ChoreInterval),
		limiter: sync2.NewLimiter(config.NumWorkers),
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer chore.limiter.Wait()
	return chore.Loop.Run(ctx, chore.AddMissing)
}

// AddMissing starts any missing satellite chore.
func (chore *Chore) AddMissing(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	geSatellites, err := chore.service.ListPendingExits(ctx)
	if err != nil {
		chore.log.Error("error retrieving satellites.", zap.Error(err))
		return nil
	}

	if len(geSatellites) == 0 {
		return nil
	}
	chore.log.Debug("exiting", zap.Int("satellites", len(geSatellites)))

	for _, satellite := range geSatellites {
		mon.Meter("satellite_gracefulexit_request").Mark(1)
		satellite := satellite

		worker := NewWorker(chore.log, chore.service, chore.dialer, satellite.NodeURL, chore.config)
		if _, ok := chore.exitingMap.LoadOrStore(satellite.SatelliteID, worker); ok {
			// already running a worker for this satellite
			chore.log.Debug("skipping for satellite, worker already exists.", zap.Stringer("Satellite ID", satellite.SatelliteID))
			continue
		}

		started := chore.limiter.Go(ctx, func() {
			defer chore.exitingMap.Delete(satellite.SatelliteID)
			if err := worker.Run(ctx); err != nil {
				chore.log.Error("worker failed", zap.Error(err))
			}
		})
		if !started {
			chore.exitingMap.Delete(satellite.SatelliteID)
			return ctx.Err()
		}
	}

	return nil
}

// TestWaitForNoWorkers waits for any pending worker to finish.
func (chore *Chore) TestWaitForNoWorkers(ctx context.Context) error {
	for {
		if !sync2.Sleep(ctx, 100*time.Millisecond) {
			return ctx.Err()
		}

		found := false
		chore.exitingMap.Range(func(key, value interface{}) bool {
			found = true
			return false
		})
		if !found {
			return nil
		}
	}
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
