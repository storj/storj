// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
)

// Config contains configurable values for repairer
type Config struct {
	MaxRepair     int           `help:"maximum segments that can be repaired concurrently" default:"100"`
	Interval      time.Duration `help:"how frequently checker should audit segments" default:"3600s"`
	OverlayAddr   string        `help:"Address to contact overlay server through"`
	PointerDBAddr string        `help:"Address to contact pointerdb server through"`
	MaxBufferMem  memory.Size   `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4M"`
}

// SegmentRepairer is a repairer for segments
type SegmentRepairer interface {
	Repair(ctx context.Context, path storj.Path, lostPieces []int32) (err error)
}

// Service contains the information needed to run the repair service
type Service struct {
	queue     queue.RepairQueue
	config    *Config
	transport transport.Client
	repairer  SegmentRepairer
	limiter   *sync2.Limiter
	ticker    *time.Ticker
	pdb       *pointerdb.Service
}

// NewService creates repairing service
func NewService(queue queue.RepairQueue, config *Config, transport transport.Client, interval time.Duration, concurrency int, pdb *pointerdb.Service) *Service {
	return &Service{
		queue:     queue,
		config:    config,
		transport: transport,
		limiter:   sync2.NewLimiter(concurrency),
		ticker:    time.NewTicker(interval),
		pdb:       pdbServer,
	}
}

// Close closes resources
func (service *Service) Close() error {
	// TODO
	// err := service.repairer.Close()
	// close queue?
	return nil
}

// Run runs the repairer service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Initialize segment repairer
	var oc overlay.Client
	oc, err = overlay.NewClientContext(ctx, service.transport, service.config.OverlayAddr)
	if err != nil {
		return err
	}
	ec := ecclient.NewClient(service.transport, service.config.MaxBufferMem.Int())

	service.repairer = segments.NewSegmentRepairer(oc, ec, service.pdb)
	defer func() { err = errs.Combine(err, service.Close()) }()

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
func (service *Service) process(ctx context.Context) error {
	seg, err := service.queue.Dequeue(ctx)
	if err != nil {
		if storage.ErrEmptyQueue.Has(err) {
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
