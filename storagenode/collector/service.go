// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package collector

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

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
		err := service.collect(ctx)
		if err != nil {
			service.log.Error("error during collecting pieces: ", zap.Error(err))
		}
		return nil
	})
}

func (service *Service) collect(ctx context.Context) (err error) {
	now := time.Now()
	const maxBatches = 100
	const batchSize = 1024

	for k := 0; k < maxBatches; k++ {
		ids, err := service.pieceinfos.GetExpired(ctx, now, batchSize)
		if err == sql.ErrNoRows {
			return nil
		} else if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}

		for _, id := range ids {
			err := service.pieces.Delete(ctx, id.SatelliteID, id.PieceID)
			if err != nil {
				service.log.Error("unable to delete piece", zap.Stringer("satellite id", id.SatelliteID), zap.Stringer("piece id", id.PieceID))
				continue
			}

			err = service.pieceinfos.DeleteExpired(ctx, now, id.SatelliteID, id.PieceID)
			if err != nil {
				service.log.Error("unable to delete piece info", zap.Stringer("satellite id", id.SatelliteID), zap.Stringer("piece id", id.PieceID))
				continue
			}
		}
	}

	return errors.New("unable to cleanup everything")
}
