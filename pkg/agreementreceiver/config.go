// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package agreementreceiver

import (
	"context"

	"go.uber.org/zap"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	mon = monkit.Package()
)

// Config is a configuration struct that is everything you need to start an
// agreement receiver responsibility
type Config struct {
	DatabaseURL string `help:"the database connection string to use" default:"$CONFDIR/agreements.db"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)
	ns, err := NewServer(c.DatabaseURL, server.Identity(), zap.L())
	if err != nil {
		return err
	}

	pb.RegisterBandwidthServer(server.GRPC(), ns)

	return server.Run(ctx)
}
