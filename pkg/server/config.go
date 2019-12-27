// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
)

// Config holds server specific configuration parameters
type Config struct {
	tlsopts.Config
	Address         string `user:"true" help:"public address to listen on" default:":7777"`
	PrivateAddress  string `user:"true" help:"private address to listen on" default:"127.0.0.1:7778"`
	DebugLogTraffic bool   `user:"true" help:"log all GRPC traffic to zap logger" default:"false"`
}

// Run will run the given responsibilities with the configured identity.
func (sc Config) Run(ctx context.Context, log *zap.Logger, identity *identity.FullIdentity, revDB extensions.RevocationDB, interceptor grpc.UnaryServerInterceptor, services ...Service) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Ensure revDB is not nil, since we call Close() below we do not want a panic
	if revDB == nil {
		return Error.New("revDB cannot be nil in call to Run")
	}

	tlsOptions, err := tlsopts.NewOptions(identity, sc.Config, revDB)
	if err != nil {
		return err
	}

	server, err := New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress, interceptor, services...)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		if closeErr := server.Close(); closeErr != nil {
			log.Sugar().Errorf("Failed to close server: %s", closeErr)
		}
	}()

	log.Sugar().Infof("Node %s started on %s", server.Identity().ID, sc.Address)
	return server.Run(ctx)
}
