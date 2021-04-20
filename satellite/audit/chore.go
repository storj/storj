// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/metainfo/metaloop"
)

// Chore populates reservoirs and the audit queue.
//
// architecture: Chore
type Chore struct {
	log    *zap.Logger
	rand   *rand.Rand
	queues *Queues
	Loop   *sync2.Cycle

	metainfoLoop *metaloop.Service
	config       Config
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, queues *Queues, metaLoop *metaloop.Service, config Config) *Chore {
	return &Chore{
		log:    log,
		rand:   rand.New(rand.NewSource(time.Now().Unix())),
		queues: queues,
		Loop:   sync2.NewCycle(config.ChoreInterval),

		metainfoLoop: metaLoop,
		config:       config,
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		// If the previously pushed queue is still waiting to be swapped in, wait.
		err = chore.queues.WaitForSwap(ctx)
		if err != nil {
			return err
		}

		collector := NewCollector(chore.config.Slots, chore.rand)
		err = chore.metainfoLoop.Join(ctx, collector)
		if err != nil {
			chore.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}

		var newQueue []Segment
		queueSegments := make(map[Segment]struct{})

		// Add reservoir segments to queue in pseudorandom order.
		for i := 0; i < chore.config.Slots; i++ {
			for _, res := range collector.Reservoirs {
				// Skip reservoir if no segment at this index.
				if len(res.Segments) <= i {
					continue
				}
				segment := res.Segments[i]
				if segment == (Segment{}) {
					continue
				}
				if _, ok := queueSegments[segment]; !ok {
					newQueue = append(newQueue, segment)
					queueSegments[segment] = struct{}{}
				}
			}
		}

		// Push new queue to queues struct so it can be fetched by worker.
		return chore.queues.Push(newQueue)
	})
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
