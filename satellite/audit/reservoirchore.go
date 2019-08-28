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

	queue *queue

	MetainfoLoop *metainfo.Loop
	Loop         sync2.Cycle
}

// NewReservoirChore instantiates ReservoirChore.
func NewReservoirChore(log *zap.Logger, auditQueue *queue, metaLoop *metainfo.Loop, config Config) *ReservoirChore {
	return &ReservoirChore{
		log:    log,
		config: config,
		rand:   rand.New(rand.NewSource(time.Now().Unix())),

		queue: auditQueue,

		MetainfoLoop: metaLoop,
		Loop:         *sync2.NewCycle(config.QueueInterval),
	}
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
