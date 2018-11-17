// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irreparabledb

import (
	"context"

	"go.uber.org/zap"

	pb "storj.io/storj/pkg/irreparabledb/proto"
	"storj.io/storj/pkg/provider"
)

// Config is a configuration struct that is everything you need to start a
// StatDB responsibility
type Config struct {
	DatabaseURL    string `help:"the database connection string to use" default:"$CONFDIR/stats.db"`
	DatabaseDriver string `help:"the database driver to use" default:"sqlite3"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	ns, err := NewServer(c.DatabaseDriver, c.DatabaseURL, zap.L())
	if err != nil {
		return err
	}

	pb.RegisterIrreparableDBServer(server.GRPC(), ns)

	return server.Run(ctx)
}
