// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package collector implements expired piece deletion from storage node.
package collector

import (
	"context"
	"time"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/storagenode/pieces"
)

var mon = monkit.Package()

// Config defines parameters for storage node Collector.
type Config struct {
	Interval time.Duration `help:"how frequently expired pieces are collected" default:"1h0m0s"`
}

// Service implements collecting expired pieces on the storage node.
type Service struct {
	log        *zap.Logger
	pieces     *pieces.Store
	pieceinfos pieces.DB

	Loop sync2.Cycle
}

// NewService creates a new collector service.
func NewService(log *zap.Logger, pieces *pieces.Store, pieceinfos pieces.DB, config Config) *Service {
	return &Service{
		log:        log,
		pieces:     pieces,
		pieceinfos: pieceinfos,
		Loop:       *sync2.NewCycle(config.Interval),
	}
}

// Run runs monitor service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.Collect(ctx, time.Now())
		if err != nil {
			service.log.Error("error during collecting pieces: ", zap.Error(err))
		}
		return nil
	})
}

// Close stops the collector service.
func (service *Service) Close() (err error) {
	service.Loop.Close()
	return nil
}

// Collect collects pieces that have expired by now.
func (service *Service) Collect(ctx context.Context, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	const maxBatches = 100
	const batchSize = 1000

	var count int64
	var bytes int64
	defer func() {
		if count > 0 {
			service.log.Info("collect", zap.Int64("count", count), zap.Stringer("size", memory.Size(bytes)))
		}
	}()

	for k := 0; k < maxBatches; k++ {
		infos, err := service.pieceinfos.GetExpired(ctx, now, batchSize)
		if err != nil {
			return err
		}
		if len(infos) == 0 {
			return nil
		}

		for _, expired := range infos {
			err := service.pieces.Delete(ctx, expired.SatelliteID, expired.PieceID)
			if err != nil {
				errfailed := service.pieceinfos.DeleteFailed(ctx, expired.SatelliteID, expired.PieceID, now)
				if err != nil {
					service.log.Error("unable to update piece info", zap.Stringer("satellite id", expired.SatelliteID), zap.Stringer("piece id", expired.PieceID), zap.Error(errfailed))
				}
				service.log.Error("unable to delete piece", zap.Stringer("satellite id", expired.SatelliteID), zap.Stringer("piece id", expired.PieceID), zap.Error(err))
				continue
			}

			err = service.pieceinfos.Delete(ctx, expired.SatelliteID, expired.PieceID)
			if err != nil {
				service.log.Error("unable to delete piece info", zap.Stringer("satellite id", expired.SatelliteID), zap.Stringer("piece id", expired.PieceID), zap.Error(err))
				continue
			}

			count++
			bytes += expired.PieceSize
		}
	}

	return nil
}
