// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package collector implements expired piece deletion from storage node.
package collector

import (
	"context"
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

	var count int64
	defer func() {
		if count > 0 {
			service.log.Info("collect", zap.Int64("count", count))
		}
	}()

	err = service.pieces.GetExpired(ctx, now, func(ctx context.Context, ei pieces.ExpiredInfo) bool {
		err := service.pieces.DeleteSkipV0(ctx, ei.SatelliteID, ei.PieceID)
		if err != nil {
			service.log.Warn("unable to delete piece", zap.Stringer("Satellite ID", ei.SatelliteID), zap.Stringer("Piece ID", ei.PieceID), zap.Error(err))
		} else {
			service.log.Debug("deleted expired piece", zap.Stringer("Satellite ID", ei.SatelliteID), zap.Stringer("Piece ID", ei.PieceID))
			count++
		}
		return true
	})
	_ = service.pieces.DeleteExpired(ctx, now)
	return errs.Wrap(err)
}
