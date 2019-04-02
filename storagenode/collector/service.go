// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package collector

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/storagenode/pieces"
)

var (
	mon = monkit.Package()

	// Error is the default error class for piecestore collector errors
	Error = errs.Class("piecestore collector")
)

// Config defines parameters for storage node Collector.
type Config struct {
	Interval time.Duration `help:"how frequently the expired pieces are cleaned" default:"0h0m5s"`
}

// Service implements collecting expired pieces on the storage node.
type Service struct {
	log        *zap.Logger
	pieces     *pieces.Store
	pieceinfos pieces.DB
	Loop       sync2.Cycle
}

// NewService creates a new collector service.
func NewService(log *zap.Logger, pieces *pieces.Store, pieceinfos pieces.DB, interval time.Duration) *Service {
	return &Service{
		log:        log,
		pieces:     pieces,
		pieceinfos: pieceinfos,
		Loop:       *sync2.NewCycle(interval),
	}
}

// Run runs the collector at regular intervals
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err = service.Collect(ctx)
		return err
	})
}

// Collect collects expired pieces at this moment.
func (service *Service) Collect(ctx context.Context) error {
	info, err := service.pieceinfos.GetExpired(ctx, time.Now())
	if err != nil {
		return Error.Wrap(err)
	}
	for _, i := range info {
		err := service.pieces.Delete(ctx, i.SatelliteID, i.PieceID)
		if err != nil {
			service.log.Error("unable to delete the piece from storage", zap.Error(err))
			continue
		}
		err = service.pieceinfos.DeleteExpired(ctx, time.Now())
		if err != nil {
			service.log.Error("unable to delete the piece info from db", zap.Error(err))
		}
	}
	return err
}
