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
	Interval              time.Duration `help:"how frequently expired pieces are collected" default:"1h0m0s"`
	ExpirationGracePeriod time.Duration `help:"how long should the collector wait before deleting expired pieces. Should not be less than 30 min since nodes are allowed to be 30 mins out of sync with the satellite." default:"1h0m0s"`
	ExpirationBatchSize   int           `help:"how many expired pieces to delete in one batch. If <= 0, all expired pieces will be deleted in one batch." default:"1000"`
}

// Service implements collecting expired pieces on the storage node.
//
// architecture: Chore
type Service struct {
	log         *zap.Logger
	pieces      *pieces.Store
	usedSerials *usedserials.Table

	Loop *sync2.Cycle

	batchSize             int
	expirationGracePeriod time.Duration
}

// NewService creates a new collector service.
func NewService(log *zap.Logger, pieces *pieces.Store, usedSerials *usedserials.Table, config Config) *Service {
	if config.ExpirationGracePeriod.Minutes() < 30 {
		log.Warn("ExpirationGracePeriod cannot not be less than 30 minutes. Using default")
		config.ExpirationGracePeriod = 1 * time.Hour
	}

	return &Service{
		log:                   log,
		pieces:                pieces,
		usedSerials:           usedSerials,
		batchSize:             config.ExpirationBatchSize,
		expirationGracePeriod: config.ExpirationGracePeriod,
		Loop:                  sync2.NewCycle(config.Interval),
	}
}

// Run runs collector service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		// V3-3143 Collection of expired pieces should be delayed after expiration
		// to avoid premature deletion due to timezone issues, which may lead to
		// storage node disqualification.
		err := service.Collect(ctx, time.Now().Add(-service.expirationGracePeriod))
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

	service.log.Info("expired pieces collection started")
	numCollected := 0
	defer func() {
		if err != nil {
			service.log.Error("error during expired pieces collection", zap.Int("count", numCollected), zap.Error(err))
		} else {
			service.log.Info("expired pieces collection completed", zap.Int("count", numCollected))
		}
	}()

	for {
		batch, err := service.pieces.GetExpiredBatchSkipV0(ctx, now, service.batchSize)
		if err != nil {
			return errs.Wrap(err)
		}

		count := len(batch)
		if count == 0 {
			return nil
		}

		for _, ei := range batch {
			// delete the piece from the storage
			err := service.pieces.DeleteSkipV0(ctx, ei.SatelliteID, ei.PieceID, ei.PieceSize)
			if err != nil {
				service.log.Warn("unable to delete piece", zap.Stringer("Satellite ID", ei.SatelliteID), zap.Stringer("Piece ID", ei.PieceID), zap.Error(err))
			} else {
				service.log.Debug("deleted expired piece", zap.Stringer("Satellite ID", ei.SatelliteID), zap.Stringer("Piece ID", ei.PieceID))
			}
		}

		numCollected += count

		// delete the batch from the database
		if deleteErr := service.pieces.DeleteExpiredBatchSkipV0(ctx, now, service.batchSize); deleteErr != nil {
			service.log.Error("error during deleting expired pieces: ", zap.Error(deleteErr))
			return errs.Wrap(deleteErr)
		}
	}
}
