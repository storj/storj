// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/storage"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("repairer error")
	mon   = monkit.Package()
)

// Config contains configurable values for repairer
type Config struct {
	MaxRepair    int           `help:"maximum segments that can be repaired concurrently" default:"10"`
	Interval     time.Duration `help:"how frequently checker should audit segments" default:"0h5m0s"`
	Timeout      time.Duration `help:"time limit for uploading repaired pieces to new storage nodes" default:"10m0s"`
	MaxBufferMem memory.Size   `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4M"`
}

// GetSegmentRepairer creates a new segment repairer from storeConfig values
func (c Config) GetSegmentRepairer(ctx context.Context, tc transport.Client, metainfo *metainfo.Service, orders *orders.Service, cache *overlay.Cache, identity *identity.FullIdentity) (ss SegmentRepairer, err error) {
	defer mon.Task()(&ctx)(&err)

	ec := ecclient.NewClient(tc, c.MaxBufferMem.Int())

	return segments.NewSegmentRepairer(metainfo, orders, cache, ec, identity, c.Timeout), nil
}

// SegmentRepairer is a repairer for segments
type SegmentRepairer interface {
	Repair(ctx context.Context, path storj.Path, lostPieces []int32) (err error)
}

// Service contains the information needed to run the repair service
type Service struct {
	queue     queue.RepairQueue
	config    *Config
	Limiter   *sync2.Limiter
	Loop      sync2.Cycle
	transport transport.Client
	metainfo  *metainfo.Service
	orders    *orders.Service
	cache     *overlay.Cache
	repairer  SegmentRepairer
}

// NewService creates repairing service
func NewService(queue queue.RepairQueue, config *Config, interval time.Duration, concurrency int, transport transport.Client, metainfo *metainfo.Service, orders *orders.Service, cache *overlay.Cache) *Service {
	return &Service{
		queue:     queue,
		config:    config,
		Limiter:   sync2.NewLimiter(concurrency),
		Loop:      *sync2.NewCycle(interval),
		transport: transport,
		metainfo:  metainfo,
		orders:    orders,
		cache:     cache,
	}
}

// Close closes resources
func (service *Service) Close() error { return nil }

// Run runs the repairer service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: close segment repairer, currently this leaks connections
	service.repairer, err = service.config.GetSegmentRepairer(
		ctx,
		service.transport,
		service.metainfo,
		service.orders,
		service.cache,
		service.transport.Identity(),
	)
	if err != nil {
		return err
	}

	// wait for all repairs to complete
	defer service.Limiter.Wait()

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.process(ctx)
		if err != nil {
			zap.L().Error("process", zap.Error(err))
		}
		return nil
	})
}

// process picks items from repair queue and spawns a repair worker
func (service *Service) process(ctx context.Context) error {
	for {
		seg, err := service.queue.Select(ctx)
		if err != nil {
			if storage.ErrEmptyQueue.Has(err) {
				return nil
			}
			return err
		}

		service.Limiter.Go(ctx, func() {
			err := service.repairer.Repair(ctx, seg.GetPath(), seg.GetLostPieces())
			if err != nil {
				zap.L().Error("repair failed", zap.Error(err))
			}
			err = service.queue.Delete(ctx, seg)
			if err != nil {
				zap.L().Error("repair delete failed", zap.Error(err))
			}
		})
	}
}
