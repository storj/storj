// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/shared/modular"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/retain"
)

// Cleanup is a service, which combines retain,TTL,trash, and runs only once (if load is not too high).
type Cleanup struct {
	log   *zap.Logger
	blobs blobstore.Blobs
	loop  *SafeLoop

	trashExpiryInterval time.Duration
	collector           *collector.Service
	trashRunner         *pieces.TrashRunOnce
	expireRunner        collector.RunOnce
	retainRunner        *retain.RunOnce
}

// NewCleanup creates a new Cleanup.
func NewCleanup(log *zap.Logger, loop *SafeLoop, blobs blobstore.Blobs, store *pieces.PieceExpirationStore, ps *pieces.Store, rc retain.Config) *Cleanup {

	noCancel := &modular.StopTrigger{
		Cancel: func() {
		},
	}
	trashRunner := pieces.NewTrashRunOnce(log, blobs, 7*24*time.Hour, noCancel)
	expireRunner := collector.NewRunnerOnce(log, store, blobs, noCancel)
	retainRunner := retain.NewRunOnce(log, ps, rc, noCancel)
	return &Cleanup{
		loop: loop,
		// TODO: change if it's configurable. Right now it's hardcoded in storagenode/peer.go, what we wouldn't like to depends on.
		trashExpiryInterval: 7 * 24 * time.Hour,
		blobs:               blobs,
		log:                 log,
		trashRunner:         trashRunner,
		expireRunner:        expireRunner,
		retainRunner:        retainRunner,
	}
}

// Run starts running RunOnce in the safe loop.
func (c *Cleanup) Run(ctx context.Context) error {
	return c.loop.RunSafe(ctx, c.RunOnce)
}

// RunOnce executes all chores, one by one. Can be stopped with cancelling context.
func (c *Cleanup) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = c.trashRunner.Run(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}

	err = c.expireRunner.Run(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}

	err = c.retainRunner.Run(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}

	return nil

}
