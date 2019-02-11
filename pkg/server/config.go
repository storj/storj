// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/utils"
)

// Config holds server specific configuration parameters
type Config struct {
	tlsopts.Config
	Address string `user:"true" help:"address to listen on" default:":7777"`
}

// Run will run the given responsibilities with the configured identity.
func (sc Config) Run(ctx context.Context, identity *identity.FullIdentity, interceptor grpc.UnaryServerInterceptor, services ...Service) (err error) {
	defer mon.Task()(&ctx)(&err)

	lis, err := net.Listen("tcp", sc.Address)
	if err != nil {
		return err
	}
	defer func() { _ = lis.Close() }()

	opts, err := tlsopts.NewOptions(identity, sc.Config)
	if err != nil {
		return err
	}
	defer func() { err = utils.CombineErrors(err, opts.RevDB.Close()) }()

	server, err := New(opts, lis, interceptor, services...)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		if closeErr := server.Close(); closeErr != nil {
			zap.S().Errorf("Failed to close server: %s", closeErr)
		}
	}()

	zap.S().Infof("Node %s started on %s", server.Identity().ID, sc.Address)
	return server.Run(ctx)
}
