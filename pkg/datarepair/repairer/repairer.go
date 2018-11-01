// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
)

// Repairer is the interface for the data repair queue
type Repairer interface {
	Repair(ctx context.Context, seg *pb.InjuredSegment) error
	Run(ctx context.Context) error
}

// repairer holds important values for data repair
type repairer struct {
	queue   queue.RepairQueue
	limiter *sync2.Limiter
	ticker  *time.Ticker
}

func newRepairer(queue queue.RepairQueue, interval time.Duration, concurrency int) *repairer {
	return &repairer{
		queue:   queue,
		limiter: sync2.NewLimiter(concurrency),
		ticker:  time.NewTicker(interval),
	}
}

// Run runs the repairer service
func (r *repairer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// wait for all repairs to complete
	defer r.limiter.Wait()

	for {
		err := r.process(ctx)
		if err != nil {
			zap.L().Error("process", zap.Error(err))
		}

		select {
		case <-r.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the repairer is canceled via context
			return ctx.Err()
		}
	}
}

// process picks an item from repair queue and spawns a repairer
func (r *repairer) process(ctx context.Context) error {
	seg, err := r.queue.Dequeue()
	if err != nil {
		// TODO: only log when err != ErrQueueEmpty
		return err
	}

	r.limiter.Go(ctx, func() {
		err := r.Repair(ctx, &seg)
		if err != nil {
			zap.L().Error("Repair failed", zap.Error(err))
		}
	})

	return nil
}

// Repair starts repair of the segment
func (r *repairer) Repair(ctx context.Context, seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO:
	zap.L().Debug("Repairing", zap.Any("segment", seg))
	return err
}
