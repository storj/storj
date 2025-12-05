// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package forgetsatellite

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// Config defines the config for the forget satellite chore.
type Config struct {
	ChoreInterval time.Duration `help:"how often to run the chore to check for satellites for the node to forget" releaseDefault:"1m" devDefault:"10s"`
	NumWorkers    int           `help:"number of workers to handle forget satellite" default:"1"`
}

// Chore checks for satellites that the node wants to forget and creates a worker per satellite to complete the process.
type Chore struct {
	log     *zap.Logger
	config  Config
	cleaner *Cleaner

	Loop *sync2.Cycle

	satelliteMap sync.Map
}

// NewChore instantiates a new forget satellite chore.
func NewChore(log *zap.Logger, cleaner *Cleaner, config Config) *Chore {
	return &Chore{
		log:     log,
		config:  config,
		cleaner: cleaner,
		Loop:    sync2.NewCycle(config.ChoreInterval),
	}
}

// Run starts the forget satellite chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, chore.RunOnce)
}

// RunOnce runs the forget satellite chore once.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	limiter := sync2.NewLimiter(chore.config.NumWorkers)
	defer limiter.Wait()

	sats, err := chore.cleaner.ListSatellites(ctx)
	if err != nil {
		return err
	}

	for _, satellite := range sats {
		worker := NewWorker(chore.log.With(zap.Stringer("satelliteID", satellite)), chore.cleaner, satellite)
		if _, ok := chore.satelliteMap.LoadOrStore(satellite, worker); ok {
			chore.log.Debug("forget-satellite already in progress", zap.Stringer("satellite", satellite))
			continue
		}

		started := limiter.Go(ctx, func() {
			defer chore.satelliteMap.Delete(satellite)
			err := worker.Run(ctx)
			if err != nil {
				chore.log.Error("error running forget-satellite worker", zap.Error(err))
			}
		})
		if !started {
			chore.satelliteMap.Delete(satellite)
			return ctx.Err()
		}
	}

	return nil
}

// Close closes the forget satellite chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
