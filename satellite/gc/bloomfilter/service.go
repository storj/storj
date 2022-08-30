// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/metabase/segmentloop"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// Config contains configurable values for garbage collection.
type Config struct {
	Interval time.Duration `help:"the time between each garbage collection executions" releaseDefault:"120h" devDefault:"10m" testDefault:"$TESTINTERVAL"`
	// TODO service is not enabled by default for testing until will be finished
	Enabled bool `help:"set if garbage collection bloom filters is enabled or not" default:"true" testDefault:"false"`

	// value for InitialPieces currently based on average pieces per node
	InitialPieces     int     `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a garbage collection bloom filter" releaseDefault:"0.1" devDefault:"0.1"`
}

// Service implements the garbage collection service.
//
// architecture: Chore
type Service struct {
	log    *zap.Logger
	config Config
	Loop   *sync2.Cycle

	overlay     overlay.DB
	segmentLoop *segmentloop.Service
}

// NewService creates a new instance of the gc service.
func NewService(log *zap.Logger, config Config, overlay overlay.DB, loop *segmentloop.Service) *Service {
	return &Service{
		log:         log,
		config:      config,
		Loop:        sync2.NewCycle(config.Interval),
		overlay:     overlay,
		segmentLoop: loop,
	}
}

// Run starts the gc loop service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.config.Enabled {
		return nil
	}

	// load last piece counts from overlay db
	lastPieceCounts, err := service.overlay.AllPieceCounts(ctx)
	if err != nil {
		service.log.Error("error getting last piece counts", zap.Error(err))
		err = nil
	}
	if lastPieceCounts == nil {
		lastPieceCounts = make(map[storj.NodeID]int)
	}

	return service.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		service.log.Debug("collecting bloom filters started")

		pieceTracker := NewPieceTracker(service.log.Named("gc observer"), service.config, lastPieceCounts)

		// collect things to retain
		err = service.segmentLoop.Join(ctx, pieceTracker)
		if err != nil {
			service.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}

		// TODO send bloom filters to the bucket

		service.log.Debug("collecting bloom filters finished")

		return nil
	})
}
