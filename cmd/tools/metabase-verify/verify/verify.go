// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package verify

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
)

// Error is the default error class for the package.
var Error = errs.Class("verify")

// Chore runs different verifications on metabase loop.
type Chore struct {
	Log *zap.Logger

	Config Config

	DB *metabase.DB
}

// Config contains configuration for all the services.
type Config struct {
	ProgressPrintFrequency int64

	Loop rangedloop.Config
}

// New creates new verification.
func New(log *zap.Logger, mdb *metabase.DB, config Config) *Chore {
	return &Chore{
		Log:    log,
		Config: config,
		DB:     mdb,
	}
}

// RunOnce creates a new rangedloop and runs the verifications.
func (chore *Chore) RunOnce(ctx context.Context) error {
	plainOffset := &SegmentSizes{
		Log: chore.Log.Named("segment-sizes"),
	}
	progress := &ProgressObserver{
		Log:                    chore.Log.Named("progress"),
		ProgressPrintFrequency: chore.Config.ProgressPrintFrequency,
	}

	// override parallelism to simulate old segments loop
	chore.Config.Loop.Parallelism = 1
	provider := rangedloop.NewMetabaseRangeSplitter(chore.Log, chore.DB, rangedloop.Config{
		AsOfSystemInterval: 5 * time.Second,
		BatchSize:          2500,
	})
	loop := rangedloop.NewService(chore.Log, chore.Config.Loop, provider,
		[]rangedloop.Observer{
			plainOffset,
			progress,
		})

	_, err := loop.RunOnce(ctx)
	return Error.Wrap(err)
}
