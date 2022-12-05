// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// Chore populates reservoirs and the audit queue.
//
// architecture: Chore
type Chore struct {
	log   *zap.Logger
	rand  *rand.Rand
	queue VerifyQueue
	Loop  *sync2.Cycle

	segmentLoop *segmentloop.Service
	config      Config
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, queue VerifyQueue, loop *segmentloop.Service, config Config) *Chore {
	if config.VerificationPushBatchSize < 1 {
		config.VerificationPushBatchSize = 1
	}
	return &Chore{
		log:   log,
		rand:  rand.New(rand.NewSource(time.Now().Unix())),
		queue: queue,
		Loop:  sync2.NewCycle(config.ChoreInterval),

		segmentLoop: loop,
		config:      config,
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		collector := NewCollector(chore.config.Slots, chore.rand)
		err = chore.segmentLoop.Join(ctx, collector)
		if err != nil {
			chore.log.Error("error joining segmentloop", zap.Error(err))
			return nil
		}

		type SegmentKey struct {
			StreamID uuid.UUID
			Position uint64
		}

		var newQueue []Segment
		queueSegments := make(map[SegmentKey]struct{})

		// Add reservoir segments to queue in pseudorandom order.
		for i := 0; i < chore.config.Slots; i++ {
			for _, res := range collector.Reservoirs {
				// Skip reservoir if no segment at this index.
				if len(res.Segments) <= i {
					continue
				}
				segment := res.Segments[i]
				segmentKey := SegmentKey{
					StreamID: segment.StreamID,
					Position: segment.Position.Encode(),
				}
				if segmentKey == (SegmentKey{}) {
					continue
				}

				if _, ok := queueSegments[segmentKey]; !ok {
					newQueue = append(newQueue, NewSegment(segment))
					queueSegments[segmentKey] = struct{}{}
				}
			}
		}

		// Push new queue to queues struct so it can be fetched by worker.
		return chore.queue.Push(ctx, newQueue, chore.config.VerificationPushBatchSize)
	})
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
