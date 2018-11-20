// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"time"

	"go.uber.org/zap"
	"storj.io/storj/pkg/provider"
)

// Config contains configurable values for rollup
type Config struct {
	Interval       time.Duration `help:"how frequently rollup should run" default:"30s"`
	DatabaseURL    string        `help:"the database connection string to use" default:"$CONFDIR/stats.db"`
	DatabaseDriver string        `help:"the database driver to use" default:"sqlite3"`
}

// Initialize a rollup struct
func (c Config) initialize(ctx context.Context) (Rollup, error) {
	return newRollup(c.DatabaseDriver, c.DatabaseURL, zap.L(), c.Interval)
}

// Run runs the rollup with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	rollup, err := c.initialize(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		if err := rollup.Run(ctx); err != nil {
			defer cancel()
			zap.L().Error("Error running rollup", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}
