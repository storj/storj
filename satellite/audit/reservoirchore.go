// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

// ReservoirChore populates reservoirs and the audit queue.
type ReservoirChore struct {
	log    *zap.Logger
	config Config
	rand   *rand.Rand
	Queue  *Queue
	Loop   sync2.Cycle

	MetainfoLoop *metainfo.Loop
}

// NewReservoirChore instantiates ReservoirChore.
func NewReservoirChore(log *zap.Logger, queue *Queue, metaLoop *metainfo.Loop, config Config) *ReservoirChore {
	return &ReservoirChore{
		log:    log,
		config: config,
		rand:   rand.New(rand.NewSource(time.Now().Unix())),
		Queue:  queue,
		Loop:   *sync2.NewCycle(config.ChoreInterval),

		MetainfoLoop: metaLoop,
	}
}

// Run starts the reservoir chore.
func (chore *ReservoirChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		pathCollector := NewPathCollector(chore.config.Slots, chore.rand)
		err = chore.MetainfoLoop.Join(ctx, pathCollector)
		if err != nil {
			chore.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}

		var newQueue []storj.Path
		queuePaths := make(map[storj.Path]struct{})

		// Add reservoir paths to queue in pseudorandom order.
		for i := 0; i < chore.config.Slots; i++ {
			for _, res := range pathCollector.Reservoirs {
				// Skip reservoir if no path at this index.
				if len(res.Paths) <= i {
					continue
				}
				path := res.Paths[i]
				if path == "" {
					continue
				}
				if _, ok := queuePaths[path]; !ok {
					newQueue = append(newQueue, path)
					queuePaths[path] = struct{}{}
				}
			}
		}
		chore.Queue.Swap(newQueue)

		return nil
	})
}

// Close closese ReservoirChore.
func (chore *ReservoirChore) Close() error {
	chore.Loop.Close()
	return nil
}
