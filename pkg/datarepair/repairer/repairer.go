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
	"storj.io/storj/pkg/datarepair/irreparable"
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
	MaxRepair        int           `help:"maximum segments that can be repaired concurrently" releaseDefault:"5" devDefault:"1"`
	RepairInterval   time.Duration `help:"how frequently repair checker should audit segments" releaseDefault:"1h" devDefault:"0h5m0s"`
	IrrepairInterval time.Duration `help:"how frequently irrepair checker should audit segments" releaseDefault:"30m" devDefault:"0h1m0s"`
	Timeout          time.Duration `help:"time limit for uploading repaired pieces to new storage nodes" default:"10m0s"`
	MaxBufferMem     memory.Size   `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4M"`
}

// GetSegmentRepairer creates a new segment repairer from storeConfig values
func (c Config) GetSegmentRepairer(ctx context.Context, tc transport.Client, metainfo *metainfo.Service, orders *orders.Service, cache *overlay.Cache, identity *identity.FullIdentity) (ss SegmentRepairer, err error) {
	defer mon.Task()(&ctx)(&err)

	ec := ecclient.NewClient(tc, c.MaxBufferMem.Int())

	return segments.NewSegmentRepairer(metainfo, orders, cache, ec, identity, c.Timeout), nil
}

// SegmentRepairer is a repairer for segments
type SegmentRepairer interface {
	Repair(ctx context.Context, path storj.Path) (err error)
}

// Service contains the information needed to run the repair service
type Service struct {
	queue        queue.RepairQueue
	irrdb        irreparable.DB
	config       *Config
	Limiter      *sync2.Limiter
	RepairLoop   sync2.Cycle
	IrrepairLoop sync2.Cycle
	transport    transport.Client
	metainfo     *metainfo.Service
	orders       *orders.Service
	cache        *overlay.Cache
	repairer     SegmentRepairer
}

// NewService creates repairing service
func NewService(queue queue.RepairQueue, irrdb irreparable.DB, config *Config, repairInterval, irrepairInterval time.Duration, concurrency int, transport transport.Client, metainfo *metainfo.Service, orders *orders.Service, cache *overlay.Cache) *Service {
	return &Service{
		queue:        queue,
		irrdb:        irrdb,
		config:       config,
		Limiter:      sync2.NewLimiter(concurrency),
		RepairLoop:   *sync2.NewCycle(repairInterval),
		IrrepairLoop: *sync2.NewCycle(irrepairInterval),
		transport:    transport,
		metainfo:     metainfo,
		orders:       orders,
		cache:        cache,
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
	c := make(chan error)

	go func() {
		c <- service.RepairLoop.Run(ctx, func(ctx context.Context) error {
			err := service.repairProcess(ctx)
			if err != nil {
				zap.L().Error("process", zap.Error(err))
			}
			return nil
		})
	}()

	go func() {
		c <- service.IrrepairLoop.Run(ctx, func(ctx context.Context) error {
			err := service.irrepairProcess(ctx)
			if err != nil {
				zap.L().Error("process", zap.Error(err))
			}
			return nil
		})
	}()

	for err := range c {
		if err != nil {
			return err
		}
	}
	return nil
}

// repairProcess picks items from repair queue and spawns a repair worker
func (service *Service) repairProcess(ctx context.Context) error {
	for {
		seg, err := service.queue.Select(ctx)
		zap.L().Info("Dequeued segment from repair queue", zap.String("segment", seg.GetPath()))
		if err != nil {
			if storage.ErrEmptyQueue.Has(err) {
				return nil
			}
			return err
		}

		service.Limiter.Go(ctx, func() {
			zap.L().Info("Limiter running repair on segment", zap.String("segment", seg.GetPath()))
			err := service.repairer.Repair(ctx, seg.GetPath())
			if err != nil {
				zap.L().Error("repair failed", zap.Error(err))
			}
			zap.L().Info("Deleting segment from repair queue", zap.String("segment", seg.GetPath()))
			err = service.queue.Delete(ctx, seg)
			if err != nil {
				zap.L().Error("repair delete failed", zap.Error(err))
			}
		})
	}
}

// irrepairProcess picks items from irrepairabledb and spawns a repair worker
func (service *Service) irrepairProcess(ctx context.Context) error {
	limit := 1
	var offset int64
	for {
		seg, err := service.irrdb.GetLimited(ctx, limit, offset)
		if err != nil {
			return err
		}

		// irrepairabledb empty
		if len(seg) == 0 {
			return nil
		}

		service.Limiter.Go(ctx, func() {
			err := service.repairer.Repair(ctx, string(seg[0].GetPath()))
			if err != nil {
				zap.L().Error("repair failed", zap.Error(err))
			}
			err = service.irrdb.Delete(ctx, seg[0].GetPath())
			if err != nil {
				zap.L().Error("repair delete failed", zap.Error(err))
			}
			offset++
		})
	}
}
