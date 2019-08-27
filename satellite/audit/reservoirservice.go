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

// ReservoirService is a temp name for the service struct during the audit 2.0 refactor.
// Once V3-2363 and V3-2364 are implemented, ReservoirService will replace the existing Service struct.
type ReservoirService struct {
	log    *zap.Logger
	config Config
	rand   *rand.Rand

	cond   sync.Cond
	queue  []storj.Path
	closed chan struct{}

	MetainfoLoop *metainfo.Loop
	Loop         sync2.Cycle
}

// NewReservoirService instantiates ReservoirService
func NewReservoirService(log *zap.Logger, metaLoop *metainfo.Loop, config Config) *ReservoirService {
	return &ReservoirService{
		log:    log,
		config: config,
		rand:   rand.New(rand.NewSource(time.Now().Unix())),

		cond: *sync.NewCond(&sync.Mutex{}),

		MetainfoLoop: metaLoop,
		Loop:         *sync2.NewCycle(config.Interval),
	}
}

// Run runs auditing service 2.0
func (service *ReservoirService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("audit 2.0 is starting up")

	var group errgroup.Group
	group.Go(func() error {
		return service.populateQueueJob(ctx)
	})

	for i := 0; i < service.config.WorkerCount; i++ {
		group.Go(func() error {
			return service.worker(ctx)
		})
	}

	return group.Wait()
}

func (service *ReservoirService) populateQueueJob(ctx context.Context) error {
	return service.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		select {
		case <-service.closed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pathCollector := NewPathCollector(service.config.Slots, service.rand)
		err = service.MetainfoLoop.Join(ctx, pathCollector)
		if err != nil {
			service.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}

		var queue []storj.Path
		queuePaths := make(map[storj.Path]bool)

		// Add reservoir paths to queue in pseudorandom order.
		for i := 0; i < service.config.Slots; i++ {
			for _, res := range pathCollector.Reservoirs {
				path := res.Paths[i]
				if !queuePaths[path] {
					queue = append(queue, path)
					queuePaths[path] = true
				}
			}
		}
		service.cond.L.Lock()
		service.queue = queue
		// Notify workers that queue has been repopulated.
		service.cond.Broadcast()
		service.cond.L.Unlock()

		return nil
	})
}

// worker removes an item from the queue and runs an audit.
func (service *ReservoirService) worker(ctx context.Context) error {
	for {
		select {
		case <-service.closed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_ = service.next()
		// TODO: audit the path
	}
}

// next gets the next item in the queue.
func (service *ReservoirService) next() storj.Path {
	service.cond.L.Lock()
	defer service.cond.L.Unlock()

	if len(service.queue) == 0 {
		// This waits until the queue is repopulated.
		service.cond.Wait()
	}
	next := service.queue[0]
	service.queue = service.queue[1:]

	return next
}

// Close halts the reservoir service loop
func (service *ReservoirService) Close() error {
	close(service.closed)
	return nil
}
