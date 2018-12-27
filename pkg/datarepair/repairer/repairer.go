// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// SegmentRepairer is a repairer for segments
type SegmentRepairer interface {
	Repair(ctx context.Context, path storj.Path, lostPieces []int32) (err error)
}

// repairService contains the information needed to run the repair service
type repairService struct {
	queue    queue.RepairQueue
	repairer SegmentRepairer
	limiter  *sync2.Limiter
	ticker   *time.Ticker
}

func newService(queue queue.RepairQueue, repairer SegmentRepairer, interval time.Duration, concurrency int) *repairService {
	return &repairService{
		queue:    queue,
		repairer: repairer,
		limiter:  sync2.NewLimiter(concurrency),
		ticker:   time.NewTicker(interval),
	}
}

// Run runs the repairer service
func (service *repairService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// wait for all repairs to complete
	defer service.limiter.Wait()

	for {
		err := service.process(ctx)
		if err != nil {
			zap.L().Error("process", zap.Error(err))
		}

		select {
		case <-service.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the repairer service is canceled via context
			return ctx.Err()
		}
	}
}

// process picks an item from repair queue and spawns a repair worker
func (service *repairService) process(ctx context.Context) error {
	seg, err := service.queue.Dequeue(ctx)
	if err != nil {
		if err == storage.ErrEmptyQueue {
			return nil
		}
		return err
	}

	service.limiter.Go(ctx, func() {
		err := service.repairer.Repair(ctx, seg.GetPath(), seg.GetLostPieces())
		if err != nil {
			zap.L().Error("Repair failed", zap.Error(err))
		}
	})

	return nil
}
