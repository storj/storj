// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retain

import (
	"context"

	"storj.io/storj/shared/modular"
)

// RunOnce is a helper to run the retain cleaner only once.
type RunOnce struct {
	service *Service
	stop    *modular.StopTrigger
}

// NewRunOnce creates a new RunOnce.
func NewRunOnce(service *Service, stop *modular.StopTrigger) *RunOnce {
	return &RunOnce{
		service: service,
		stop:    stop,
	}
}

// Run picks next saved BF, and executes retainPieces.
func (r *RunOnce) Run(ctx context.Context) error {
	req, found := r.service.queue.Next()
	if !found {
		r.service.log.Info("no more BFs to process")
		r.stop.Cancel()
		return nil
	}
	err := r.service.retainPieces(ctx, req)
	if err != nil {
		return err
	}
	_ = r.service.queue.Remove(req)
	r.stop.Cancel()
	return nil
}
