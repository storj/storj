// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/provider"
)

// Config is a configuration struct that is everything you need to start a
// StatDB responsibility
type Config struct {
	DatabaseURL    string `help:"the database connection string to use" default:"sqlite3://$CONFDIR/stats.db"`
	DatabaseDriver string `help:"the database driver to use" default:"sqlite3"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	_, err := NewServer(c.DatabaseDriver, c.DatabaseURL, zap.L())
	if err != nil {
		return err
	}

	// TODO(moby) defer closing server?
	return server.Run(ctx)
}
