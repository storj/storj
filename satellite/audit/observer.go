// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

// Observer populates reservoirs and the audit queue.
//
// architecture: Observer
type Observer struct {
	log      *zap.Logger
	queue    VerifyQueue
	config   Config
	seedRand *rand.Rand

	// The follow fields are reset on each segment loop cycle.
	reservoirs map[metabase.NodeAlias]*Reservoir
}

// NewObserver instantiates Observer.
func NewObserver(log *zap.Logger, queue VerifyQueue, config Config) *Observer {
	if config.VerificationPushBatchSize < 1 {
		config.VerificationPushBatchSize = 1
	}
	return &Observer{
		log:      log,
		queue:    queue,
		config:   config,
		seedRand: rand.New(rand.NewSource(time.Now().Unix())),
	}
}

// Start prepares the observer for audit segment collection.
func (obs *Observer) Start(ctx context.Context, startTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	obs.reservoirs = make(map[metabase.NodeAlias]*Reservoir)
	return nil
}

// Fork returns a new audit reservoir collector for the range.
func (obs *Observer) Fork(ctx context.Context) (_ rangedloop.Partial, err error) {
	defer mon.Task()(&ctx)(&err)

	// Each collector needs an RNG for sampling. On systems where time
	// resolution is low (e.g. windows is 15ms), seeding an RNG using the
	// current time (even with nanosecond precision) may end up reusing a seed
	// for two or more RNGs. To prevent that, the observer itself uses an RNG
	// to seed the per-collector RNGs.
	rnd := rand.New(rand.NewSource(obs.seedRand.Int63()))
	return NewCollector(obs.config.Slots, rnd), nil
}

// Join merges the audit reservoir collector into the per-node reservoirs.
func (obs *Observer) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)

	collector, ok := partial.(*Collector)
	if !ok {
		return errs.New("expected partial type %T but got %T", collector, partial)
	}

	for nodeAlias, reservoir := range collector.Reservoirs {
		existing, ok := obs.reservoirs[nodeAlias]
		if !ok {
			obs.reservoirs[nodeAlias] = reservoir
			continue
		}
		if err := existing.Merge(reservoir); err != nil {
			return err
		}
	}
	return nil
}

// Finish builds and dedups an audit queue from the merged per-node reservoirs.
func (obs *Observer) Finish(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	type SegmentKey struct {
		StreamID uuid.UUID
		Position uint64
	}

	var newQueue []Segment
	queueSegments := make(map[SegmentKey]struct{})

	// Add reservoir segments to queue in pseudorandom order.
	for i := 0; i < obs.config.Slots; i++ {
		for _, res := range obs.reservoirs {
			segments := res.Segments()
			// Skip reservoir if no segment at this index.
			if len(segments) <= i {
				continue
			}
			segment := segments[i]
			segmentKey := SegmentKey{
				StreamID: segment.StreamID,
				Position: segment.Position.Encode(),
			}
			if _, ok := queueSegments[segmentKey]; !ok {
				newQueue = append(newQueue, NewSegment(segment))
				queueSegments[segmentKey] = struct{}{}
			}
		}
	}

	// Push new queue to queues struct so it can be fetched by worker.
	return obs.queue.Push(ctx, newQueue, obs.config.VerificationPushBatchSize)
}
