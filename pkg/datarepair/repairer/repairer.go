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
	segment "storj.io/storj/pkg/storage/segments"
)

// Repairer is the interface for the data repairer
type RepairerI interface {
	Repair(ctx context.Context, seg *pb.InjuredSegment) error
	Run(ctx context.Context) error
}

// repairer holds important values for data repair
type Repairer struct {
	Queue   queue.RepairQueue
	Store   segment.Store
	Limiter *sync2.Limiter
	Ticker  *time.Ticker
}

func NewRepairer(queue queue.RepairQueue, ss segment.Store, interval time.Duration, concurrency int) *Repairer {
	return &Repairer{
		Queue:   queue,
		Store:   ss,
		Limiter: sync2.NewLimiter(concurrency),
		Ticker:  time.NewTicker(interval),
	}
}

// Run runs the repairer service
func (r *Repairer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// wait for all repairs to complete
	defer r.Limiter.Wait()

	for {
		err := r.process(ctx)
		if err != nil {
			zap.L().Error("process", zap.Error(err))
		}

		select {
		case <-r.Ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the repairer is canceled via context
			return ctx.Err()
		}
	}
}

// process picks an item from repair queue and spawns a repairer
func (r *Repairer) process(ctx context.Context) error {
	seg, err := r.Queue.Dequeue()
	if err != nil {
		// TODO: only log when err != ErrQueueEmpty
		return err
	}

	r.Limiter.Go(ctx, func() {
		err := r.Store.Repair(ctx, seg.GetPath(), seg.GetLostPieces())
		if err != nil {
			zap.L().Error("Repair failed", zap.Error(err))
		}
	})

	return nil
}
