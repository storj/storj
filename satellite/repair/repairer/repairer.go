// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/storage"
	"storj.io/storj/uplink/ecclient"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("repairer error")
	mon   = monkit.Package()
)

// Config contains configurable values for repairer
type Config struct {
	MaxRepair                     int           `help:"maximum segments that can be repaired concurrently" releaseDefault:"5" devDefault:"1"`
	Interval                      time.Duration `help:"how frequently repairer should try and repair more data" releaseDefault:"1h" devDefault:"0h5m0s"`
	Timeout                       time.Duration `help:"time limit for uploading repaired pieces to new storage nodes" devDefault:"10m0s" releaseDefault:"2h"`
	MaxBufferMem                  memory.Size   `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4M"`
	MaxExcessRateOptimalThreshold float64       `help:"ratio applied to the optimal threshold to calculate the excess of the maximum number of repaired pieces to upload" default:"0.05"`
}

// Service contains the information needed to run the repair service
type Service struct {
	log      *zap.Logger
	queue    queue.RepairQueue
	config   *Config
	Limiter  *sync2.Limiter
	Loop     sync2.Cycle
	repairer *SegmentRepairer
}

// NewService creates repairing service
func NewService(log *zap.Logger, queue queue.RepairQueue, config *Config, interval time.Duration, concurrency int, transport transport.Client, metainfo *metainfo.Service, orders *orders.Service, cache *overlay.Cache) *Service {
	client := ecclient.NewClient(log.Named("ecclient"), transport, config.MaxBufferMem.Int())
	repairer := NewSegmentRepairer(log.Named("repairer"), metainfo, orders, cache, client, config.Timeout, config.MaxExcessRateOptimalThreshold)

	return &Service{
		log:      log,
		queue:    queue,
		config:   config,
		Limiter:  sync2.NewLimiter(concurrency),
		Loop:     *sync2.NewCycle(interval),
		repairer: repairer,
	}
}

// Close closes resources
func (service *Service) Close() error { return nil }

// Run runs the repairer service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// wait for all repairs to complete
	defer service.Limiter.Wait()

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.process(ctx)
		if err != nil {
			service.log.Error("process", zap.Error(Error.Wrap(err)))
		}
		return nil
	})
}

// process picks items from repair queue and spawns a repair worker
func (service *Service) process(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	for {
		seg, err := service.queue.Select(ctx)
		service.log.Info("Retrieved segment from repair queue", zap.Binary("segment", seg.GetPath()))
		if err != nil {
			if storage.ErrEmptyQueue.Has(err) {
				return nil
			}
			return err
		}

		service.Limiter.Go(ctx, func() {
			err := service.worker(ctx, seg)
			if err != nil {
				service.log.Error("repair worker failed:", zap.Error(err))
			}
		})
	}
}

func (service *Service) worker(ctx context.Context, seg *pb.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)

	workerStartTime := time.Now().UTC()

	service.log.Info("Limiter running repair on segment", zap.Binary("segment", seg.GetPath()))
	err = service.repairer.Repair(ctx, string(seg.GetPath()))
	if err != nil {
		if IrreparableError.Has(err) {
			service.log.Error(
				"deleting irreparable segment from the queue:",
				zap.Error(service.queue.Delete(ctx, seg)),
				zap.Binary("segment", seg.GetPath()),
			)
		}

		return Error.New("repairing injured segment: %v", err)
	}

	service.log.Info("Deleting segment from repair queue", zap.Binary("segment", seg.GetPath()))
	err = service.queue.Delete(ctx, seg)
	if err != nil {
		return Error.New("deleting repaired segment from the queue: %v", err)
	}

	repairedTime := time.Now().UTC()
	timeForRepair := repairedTime.Sub(workerStartTime)
	mon.FloatVal("time_for_repair").Observe(timeForRepair.Seconds())

	insertedTime := seg.GetInsertedTime()
	// do not send metrics if segment was added before the InsertedTime field was added
	if !insertedTime.IsZero() {
		timeSinceQueued := workerStartTime.Sub(insertedTime)
		mon.FloatVal("time_since_checker_queue").Observe(timeSinceQueued.Seconds())
	}

	return nil
}
