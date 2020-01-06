// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// ErrVersion defines version service error.
var ErrVersion = errs.Class("version service error")

// VersionService updates conversion rates in a loop.
//
// architecture: Service
type VersionService struct {
	log     *zap.Logger
	service *Service
	Cycle   sync2.Cycle
}

// NewVersionService creates new instance of VersionService.
func NewVersionService(log *zap.Logger, service *Service, interval time.Duration) *VersionService {
	return &VersionService{
		log:     log,
		service: service,
		Cycle:   *sync2.NewCycle(interval),
	}
}

// Run runs loop which updates conversion rates for service.
func (version *VersionService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return ErrVersion.Wrap(version.Cycle.Run(ctx,
		func(ctx context.Context) error {
			version.log.Debug("running conversion rates update cycle")

			if err := version.service.UpdateRates(ctx); err != nil {
				version.log.Error("conversion rates update cycle failed", zap.Error(ErrChore.Wrap(err)))
			}

			return nil
		},
	))
}

// Close closes underlying cycle.
func (version *VersionService) Close() (err error) {
	defer mon.Task()(nil)(&err)

	version.Cycle.Close()
	return nil
}
