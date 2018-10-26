// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementreceiver

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

// Config is a configuration struct that is everything you need to start an
// agreement receiver responsibility
type Config struct {
	DatabaseURL string `help:"the database connection string to use" default:"$CONFDIR/agreements.db"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	ns, err := NewServer(c.DatabaseURL, zap.L())
	if err != nil {
		return err
	}

	pb.RegisterBandwidthServer(server.GRPC(), ns)

	return server.Run(ctx)
}
