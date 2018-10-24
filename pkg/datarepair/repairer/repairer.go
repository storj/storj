// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	q "storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/pb"
)

// Repairer is the interface for the data repair queue
type Repairer interface {
	Repair(seg *pb.InjuredSegment) error
	Run() error
}

// repairer holds important values for data repair
type repairer struct {
	ctx     context.Context
	queue   q.RepairQueue
	limiter *sync2.Limiter
	ticker  *time.Ticker
}

func newRepairer(ctx context.Context, queue q.RepairQueue, interval time.Duration, concurrency int) *repairer {
	return &repairer{
		ctx:     ctx,
		queue:   queue,
		limiter: sync2.NewLimiter(concurrency),
		ticker:  time.NewTicker(interval),
	}
}

// Run the repairer loop
func (r *repairer) Run() (err error) {
	defer mon.Task()(&r.ctx)(&err)

	// wait for all repairs to complete
	defer r.limiter.Wait()

	for {
		select {
		case <-r.ticker.C: // wait for the next interval to happen
		case <-r.ctx.Done(): // or the repairer is canceled via context
			return r.ctx.Err()
		}

		seg, err := r.queue.Dequeue()
		if err != nil {
			// TODO: only log when err != ErrQueueEmpty
			zap.L().Error("dequeue", zap.Error(err))
			continue
		}

		r.limiter.Go(r.ctx, func() {
			err := r.Repair(&seg)
			if err != nil {
				zap.L().Error("Repair failed", zap.Error(err))
			}
		})
	}
}

// Repair starts repair of the segment
func (r *repairer) Repair(seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&r.ctx)(&err)
	// TODO:
	zap.L().Debug("Repairing", zap.Any("segment", seg))
	return err
}
