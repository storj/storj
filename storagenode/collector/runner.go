// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package collector

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/shared/modular"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore/usedserials"
)

// RunOnce executes the collector only once.
type RunOnce struct {
	service *Service
	stop    *modular.StopTrigger
}

// NewRunnerOnce creates a new RunOnce.
func NewRunnerOnce(log *zap.Logger, pieces *pieces.Store, usedSerials *usedserials.Table, config Config, stop *modular.StopTrigger) RunOnce {
	service := NewService(log, pieces, usedSerials, config)
	return RunOnce{
		service: service,
		stop:    stop,
	}
}

// Run implements Runner interface.
func (r RunOnce) Run(ctx context.Context) error {
	err := r.service.Collect(ctx, time.Now())
	r.stop.Cancel()
	return err
}
