// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package forgetsatellite

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/storj"
)

// Worker is responsible for completing the cleanup for a given satellite.
type Worker struct {
	log *zap.Logger

	cleaner *Cleaner

	satellite storj.NodeID
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, cleaner *Cleaner, satellite storj.NodeID) *Worker {
	return &Worker{
		log:       log,
		cleaner:   cleaner,
		satellite: satellite,
	}
}

// Run starts the cleanup process for a satellite.
func (w *Worker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	w.log.Debug("worker started")
	defer w.log.Debug("worker finished")

	return w.cleaner.Run(ctx, w.satellite)
}
