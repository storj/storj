// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

// ReservoirChore populates reservoirs and the audit queue.
type ReservoirChore struct {
	log    *zap.Logger
	config Config
	rand   *rand.Rand

	queue *queue

	MetainfoLoop *metainfo.Loop
	Loop         sync2.Cycle
}

// NewReservoirChore instantiates ReservoirChore.
func NewReservoirChore(log *zap.Logger, metaLoop *metainfo.Loop, config Config) *ReservoirChore {
	return &ReservoirChore{
		log:    log,
		config: config,
		rand:   rand.New(rand.NewSource(time.Now().Unix())),

		queue: newQueue(*sync.NewCond(&sync.Mutex{}), make(chan struct{})),

		MetainfoLoop: metaLoop,
		Loop:         *sync2.NewCycle(config.Interval),
	}
}

// Run runs auditing service 2.0.
func (chore *ReservoirChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	chore.log.Info("audit 2.0 is starting up")

	var group errgroup.Group
	group.Go(func() error {
		return chore.populateQueueJob(ctx)
	})

	for i := 0; i < chore.config.WorkerCount; i++ {
		group.Go(func() error {
			return chore.worker(ctx)
		})
	}

	return group.Wait()
}

func (chore *ReservoirChore) populateQueueJob(ctx context.Context) error {
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		select {
		case <-chore.queue.closed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pathCollector := NewPathCollector(chore.config.Slots, chore.rand)
		err = chore.MetainfoLoop.Join(ctx, pathCollector)
		if err != nil {
			chore.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}

		var queue []storj.Path
		queuePaths := make(map[storj.Path]struct{})

		// Add reservoir paths to queue in pseudorandom order.
		for i := 0; i < chore.config.Slots; i++ {
			for _, res := range pathCollector.Reservoirs {
				path := res.Paths[i]
				if _, ok := queuePaths[path]; !ok {
					queue = append(queue, path)
					queuePaths[path] = struct{}{}
				}
			}
		}
		chore.queue.swap(queue)

		return nil
	})
}

// worker removes an item from the queue and runs an audit.
func (chore *ReservoirChore) worker(ctx context.Context) error {
	for {
		_, err := chore.queue.next()
		if err != nil {
			return err
		}
		// TODO: audit the path
	}
}

// Close halts the reservoir service loop.
func (chore *ReservoirChore) Close() error {
	close(chore.queue.closed)
	return nil
}
