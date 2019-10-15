// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/satellites"
)

// Worker is responsible for completing the graceful exit for a given satellite.
type Worker struct {
	log         *zap.Logger
	satelliteID storj.NodeID
	satelliteDB satellites.DB
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, satelliteDB satellites.DB, satelliteID storj.NodeID) *Worker {
	return &Worker{
		log:         log,
		satelliteID: satelliteID,
		satelliteDB: satelliteDB,
	}
}

// Run calls the satellite endpoint, transfers pieces, validates, and responds with success or failure.
// It also marks the satellite finished once all the pieces have been transferred
func (worker *Worker) Run(ctx context.Context, satelliteID storj.NodeID, done func()) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer done()
	worker.log.Debug("running worker")

	// TODO actually process the order limits
	// https://storjlabs.atlassian.net/browse/V3-2613

	err = worker.satelliteDB.CompleteGracefulExit(ctx, satelliteID, time.Now(), satellites.ExitSucceeded, []byte{})
	return errs.Wrap(err)
}

// Close halts the worker.
func (worker *Worker) Close() error {
	// TODO not sure this is needed yet.
	return nil
}
