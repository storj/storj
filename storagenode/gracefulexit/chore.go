// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// Chore checks for satellites that the node is exiting and creates a worker per satellite to complete the process.
//
// architecture: Chore
type Chore struct {
	log         *zap.Logger
	store       *pieces.Store
	satelliteDB satellites.DB
	trust       *trust.Pool
	dialer      rpc.Dialer

	config Config

	exitingMap sync.Map
	Loop       *sync2.Cycle
	limiter    *sync2.Limiter
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, config Config, store *pieces.Store, trust *trust.Pool, dialer rpc.Dialer, satelliteDB satellites.DB) *Chore {
	return &Chore{
		log:         log,
		store:       store,
		satelliteDB: satelliteDB,
		trust:       trust,
		dialer:      dialer,
		config:      config,
		Loop:        sync2.NewCycle(config.ChoreInterval),
		limiter:     sync2.NewLimiter(config.NumWorkers),
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		satellites, err := chore.satelliteDB.ListGracefulExits(ctx)
		if err != nil {
			chore.log.Error("error retrieving satellites.", zap.Error(err))
			return nil
		}

		if len(satellites) == 0 {
			return nil
		}
		chore.log.Debug("exiting", zap.Int("satellites", len(satellites)))

		for _, satellite := range satellites {
			mon.Meter("satellite_gracefulexit_request").Mark(1) //mon:locked
			if satellite.FinishedAt != nil {
				continue
			}

			nodeurl, err := chore.trust.GetNodeURL(ctx, satellite.SatelliteID)
			if err != nil {
				chore.log.Error("failed to get satellite address.", zap.Error(err))
				continue
			}

			worker := NewWorker(chore.log, chore.store, chore.trust, chore.satelliteDB, chore.dialer, nodeurl, chore.config)
			if _, ok := chore.exitingMap.LoadOrStore(nodeurl.ID, worker); ok {
				// already running a worker for this satellite
				chore.log.Debug("skipping for satellite, worker already exists.", zap.Stringer("Satellite ID", nodeurl.ID))
				continue
			}

			chore.limiter.Go(ctx, func() {
				err := worker.Run(ctx, func() {
					chore.log.Debug("finished for satellite.", zap.Stringer("Satellite ID", nodeurl.ID))
					chore.exitingMap.Delete(nodeurl.ID)
				})

				if err != nil {
					chore.log.Error("worker failed", zap.Error(err))
				}

				if err := worker.Close(); err != nil {
					chore.log.Error("closing worker failed", zap.Error(err))
				}
			})
		}

		return nil
	})

	chore.limiter.Wait()

	return err
}

// TestWaitForWorkers waits for any pending worker to finish.
func (chore *Chore) TestWaitForWorkers() {
	chore.limiter.Wait()
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	chore.exitingMap.Range(func(key interface{}, value interface{}) bool {
		worker := value.(*Worker)
		err := worker.Close()
		if err != nil {
			worker.log.Error("worker failed on close.", zap.Error(err))
		}
		chore.exitingMap.Delete(key)
		return true
	})

	return nil
}
