// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/shared/modular"
	"storj.io/storj/storagenode/pieces"
)

// RunOnce is a helper to run the retain cleaner only once.
type RunOnce struct {
	service *Service
	stop    *modular.StopTrigger
	log     *zap.Logger
}

// NewRunOnce creates a new RunOnce.
func NewRunOnce(log *zap.Logger, ps *pieces.Store, rc Config, stop *modular.StopTrigger) *RunOnce {
	// We create a new service here, instead of using a dependency, as the Run() method of the dependencies automatically started.
	rs := NewService(log, ps, rc)
	return &RunOnce{
		log:     log,
		service: rs,
		stop:    stop,
	}
}

// Run picks next saved BF, and executes retainPieces.
func (r *RunOnce) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	req, found := r.service.queue.Next()
	if !found {
		r.service.log.Info("no more BFs to process")
		r.stop.Cancel()
		return nil
	}
	err = r.service.retainPieces(ctx, req)
	if err != nil {
		return err
	}
	_ = r.service.queue.DeleteCache(req)
	r.stop.Cancel()
	return nil
}
