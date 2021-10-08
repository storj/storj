// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package verify

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// Error is the default error class for the package.
var Error = errs.Class("verify")

// Chore runs different verifications on metabase loop.
type Chore struct {
	Log *zap.Logger

	Config Config

	DB segmentloop.MetabaseDB
}

// Config contains configuration for all the services.
type Config struct {
	ProgressPrintFrequency int64
	Loop                   segmentloop.Config
}

// New creates new verification.
func New(log *zap.Logger, mdb segmentloop.MetabaseDB, config Config) *Chore {
	return &Chore{
		Log:    log,
		Config: config,
		DB:     mdb,
	}
}

// RunOnce creates a new segmentloop and runs the verifications.
func (chore *Chore) RunOnce(ctx context.Context) error {
	loop := segmentloop.New(chore.Log, chore.Config.Loop, chore.DB)

	var group errs2.Group
	group.Go(func() error {
		plainOffset := &SegmentSizes{
			Log: chore.Log.Named("segment-sizes"),
		}
		err := loop.Join(ctx, plainOffset)
		return Error.Wrap(err)
	})

	group.Go(func() error {
		progress := &ProgressObserver{
			Log:                    chore.Log.Named("progress"),
			ProgressPrintFrequency: chore.Config.ProgressPrintFrequency,
		}
		err := loop.Monitor(ctx, progress)
		progress.Report()
		return Error.Wrap(err)
	})
	group.Go(func() error {
		return Error.Wrap(loop.RunOnce(ctx))
	})
	return Error.Wrap(errs.Combine(group.Wait()...))
}
