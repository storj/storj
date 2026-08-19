// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"
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
	return ObserverError(durations)
}

// ObserverError returns an error for every observer that failed during the run.
// The ranged loop itself only logs observer failures, so a job that runs the
// loop once would otherwise report success after producing nothing.
//
// LiveCountObserver is left out: its drift check compares estimated table
// statistics against an exact scan of a snapshot, and of a different table
// than the scan read where the source is the Avro export, so it reports a
// suspicion rather than a failed run. Failing the run on it would have a job
// whose output is fine rescan the whole metabase and publish it again.
func ObserverError(durations []ObserverDuration) error {
	var group errs.Group
	for _, duration := range durations {
		if _, advisory := duration.Observer.(*LiveCountObserver); advisory {
			continue
		}
		if duration.Err != nil {
			group.Add(errs.New("observer %T failed: %w", duration.Observer, duration.Err))
		}
	}
	return group.Err()
}
