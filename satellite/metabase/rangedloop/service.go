// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/storj/satellite/metabase/segmentloop"
)

var (
	mon = monkit.Package()
)

// Config contains configurable values for the shared loop.
type Config struct{}

// Service iterates through all segments and calls the attached observers for every segment
//
// architecture: Service
type Service struct {
	log        *zap.Logger
	config     Config
	metabaseDB segmentloop.MetabaseDB
	observers  []Observer
}

// NewService creates a new instance of the ranged loop service.
func NewService(log *zap.Logger, config Config, metabaseDB segmentloop.MetabaseDB, observers []Observer) *Service {
	return &Service{
		log:        log,
		config:     config,
		metabaseDB: metabaseDB,
		observers:  observers,
	}
}

// Run starts the looping service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		if err := service.RunOnce(ctx); err != nil {
			service.log.Error("ranged loop failure", zap.Error(err))

			if errs2.IsCanceled(err) {
				return err
			}
			if ctx.Err() != nil {
				return errs.Combine(err, ctx.Err())
			}

			mon.Event("rangedloop_error") //mon:locked
		}
	}
}

// RunOnce goes through one time and sends information to observers.
func (service *Service) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO

	return nil
}
