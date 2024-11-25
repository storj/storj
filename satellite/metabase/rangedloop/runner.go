// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"storj.io/storj/shared/modular"
)

// RunOnce is a helper to run the ranged loop only once.
type RunOnce struct {
	*Service
	stop *modular.StopTrigger
	log  *zap.Logger
}

// NewRunOnce creates a new RunOnce.
func NewRunOnce(log *zap.Logger, stop *modular.StopTrigger, config Config, provider RangeSplitter, observers []Observer) *RunOnce {
	rls := NewService(log, config, provider, observers)
	return &RunOnce{
		log:     log,
		Service: rls,
		stop:    stop,
	}
}

// Run executes ranged loop only once.
func (r *RunOnce) Run(ctx context.Context) error {
	defer func() {
		r.stop.Cancel()
	}()
	durations, err := r.Service.RunOnce(ctx)
	if err != nil {
		return err
	}
	for _, duration := range durations {
		r.log.Info("Ranged-loop observer finished",
			zap.Duration("duration", duration.Duration),
			zap.String("observer", fmt.Sprintf("%T", duration.Observer)))
	}
	return nil
}
