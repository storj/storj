// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/provider"
)

// Config contains configurable values for rollup
type Config struct {
	Interval    time.Duration `help:"how frequently rollup should run" default:"30s"`
	DatabaseURL string        `help:"the database connection string to use" default:"sqlite3://$CONFDIR/stats.db"`
}

// Initialize a rollup struct
func (c Config) initialize(ctx context.Context) (Rollup, error) {
	db, err := accounting.NewDb(c.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return newRollup(zap.L(), db, c.Interval)
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
