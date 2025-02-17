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

// Config is the config for the Cleanup.
type Config struct {
	Trash       bool `help:"enable/disable trash cleanup phase" default:"true"`
	Expire      bool `help:"enable/disable TTL expired pieces collection phase" default:"true"`
	DeleteEmpty bool `help:"enable/disable deletion of empty directories and zero sized files" default:"false"`
	Retain      bool `help:"enable/disable garbage collection phase" default:"true"`
}

// Cleanup is a service, which combines retain,TTL,trash, and runs only once (if load is not too high).
type Cleanup struct {
	log   *zap.Logger
	blobs blobstore.Blobs
	loop  *SafeLoop

	trashExpiryInterval time.Duration
	trashRunner         *pieces.TrashRunOnce
	expireRunner        collector.RunOnce
	retainRunner        *retain.RunOnce
	deleteEmptyRunner   *DeleteEmpty
	config              Config
}

// NewCleanup creates a new Cleanup.
func NewCleanup(log *zap.Logger, loop *SafeLoop, deleteEmpty *DeleteEmpty, blobs blobstore.Blobs, store *pieces.PieceExpirationStore, ps *pieces.Store, rc retain.Config, config Config) *Cleanup {

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
		deleteEmptyRunner:   deleteEmpty,
		config:              config,
	}
}

// Run starts running RunOnce in the safe loop.
func (c *Cleanup) Run(ctx context.Context) error {
	return c.loop.RunSafe(ctx, c.RunOnce)
}

// RunOnce executes all chores, one by one. Can be stopped with cancelling context.
func (c *Cleanup) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if c.config.Trash {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = c.trashRunner.Run(ctx)
		if err != nil {
			return err
		}
	}

	if c.config.Expire {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = c.expireRunner.Run(ctx)
		if err != nil {
			return err
		}
	}

	if c.config.DeleteEmpty {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := c.deleteEmptyRunner.Delete(ctx)
		if err != nil {
			return err
		}
	}
	if c.config.Retain {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = c.retainRunner.Run(ctx)
		if err != nil {
			return err
		}
	}

	return nil

}
