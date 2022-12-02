// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package collector implements expired piece deletion from storage node.
package collector

import (
	"context"
	"os"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore/usedserials"
)

var mon = monkit.Package()

// Config defines parameters for storage node Collector.
type Config struct {
	Interval time.Duration `help:"how frequently expired pieces are collected" default:"1h0m0s"`
}

// Service implements collecting expired pieces on the storage node.
//
// architecture: Chore
type Service struct {
	log         *zap.Logger
	pieces      *pieces.Store
	usedSerials *usedserials.Table

	Loop *sync2.Cycle
}

// NewService creates a new collector service.
func NewService(log *zap.Logger, pieces *pieces.Store, usedSerials *usedserials.Table, config Config) *Service {
	return &Service{
		log:         log,
		pieces:      pieces,
		usedSerials: usedSerials,
		Loop:        sync2.NewCycle(config.Interval),
	}
}

// Run runs collector service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		// V3-3143 Pieces should be collected at least 24 hours after expiration
		// to avoid premature deletion due to timezone issues, which may lead to
		// storage node disqualification.
		err := service.Collect(ctx, time.Now().Add(-24*time.Hour))
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

	service.usedSerials.DeleteExpired(now)

	const maxBatches = 100
	const batchSize = 1000

	var count int64
	defer func() {
		if count > 0 {
			service.log.Info("collect", zap.Int64("count", count))
		}
	}()

	for k := 0; k < maxBatches; k++ {
		infos, err := service.pieces.GetExpired(ctx, now, batchSize)
		if err != nil {
			return err
		}
		if len(infos) == 0 {
			return nil
		}

		for _, expired := range infos {
			err := service.pieces.Delete(ctx, expired.SatelliteID, expired.PieceID)
			if err != nil {
				if errs.Is(err, os.ErrNotExist) {
					service.log.Warn("file does not exist", zap.Stringer("Satellite ID", expired.SatelliteID), zap.Stringer("Piece ID", expired.PieceID))
					err := service.pieces.DeleteExpired(ctx, expired.SatelliteID, expired.PieceID)
					if err != nil {
						service.log.Error("unable to delete expired piece info from DB", zap.Stringer("Satellite ID", expired.SatelliteID), zap.Stringer("Piece ID", expired.PieceID), zap.Error(err))
						continue
					}
					service.log.Info("deleted expired piece info from DB", zap.Stringer("Satellite ID", expired.SatelliteID), zap.Stringer("Piece ID", expired.PieceID))
					continue
				}
				errfailed := service.pieces.DeleteFailed(ctx, expired, now)
				if errfailed != nil {
					service.log.Error("unable to update piece info", zap.Stringer("Satellite ID", expired.SatelliteID), zap.Stringer("Piece ID", expired.PieceID), zap.Error(errfailed))
				}
				service.log.Error("unable to delete piece", zap.Stringer("Satellite ID", expired.SatelliteID), zap.Stringer("Piece ID", expired.PieceID), zap.Error(err))
				continue
			}
			service.log.Info("deleted expired piece", zap.Stringer("Satellite ID", expired.SatelliteID), zap.Stringer("Piece ID", expired.PieceID))

			count++
		}
	}

	return nil
}
