// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
)

// Config contains configurable values for checker
type Config struct {
	QueueAddress     string        `help:"data checker queue address" default:"sqlite3://$CONFDIR/repairqueue.db"`
	Interval         time.Duration `help:"how frequently checker should audit segments" default:"30s"`
	IrreparabledbURL string        `help:"the database connection string to use" default:"sqlite3://$CONFDIR/irreparable.db"`
}

// Initialize a Checker struct
func (c Config) initialize(ctx context.Context) (Checker, error) {
	pdb := pointerdb.LoadFromContext(ctx)
	if pdb == nil {
		return nil, Error.New("failed to load pointerdb from context")
	}

	sdb, ok := ctx.Value("masterdb").(interface {
		StatDB() statdb.DB
	})
	if !ok {
		return nil, Error.New("unable to get master db instance")
	}

	irdb, ok := ctx.Value("masterdb").(interface {
		Irreparable() irreparable.DB
	})
	if !ok {
		return nil, Error.New("unable to get master db instance")
	}
	qdb, ok := ctx.Value("masterdb").(interface {
		RepairQueueDB() queue.RepairQueue
	})
	if !ok {
		return nil, Error.New("unable to get master db instance")
	}

	o := overlay.LoadServerFromContext(ctx)

	return newChecker(pdb, sdb.StatDB(), qdb.RepairQueueDB(), o, irdb.Irreparable(), 0, zap.L(), c.Interval), nil
}

// Run runs the checker with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	check, err := c.initialize(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		if err := check.Run(ctx); err != nil {
			defer cancel()
			zap.L().Error("Error running checker", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}
