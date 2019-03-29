// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
)

// Config holds server specific configuration parameters
type Config struct {
	tlsopts.Config
	Address        string `user:"true" help:"public address to listen on" default:":7777"`
	PrivateAddress string `user:"true" help:"private address to listen on" default:"127.0.0.1:7778"`
}

// Run will run the given responsibilities with the configured identity.
func (sc Config) Run(ctx context.Context, identity *identity.FullIdentity, interceptor grpc.UnaryServerInterceptor, services ...Service) (err error) {
	defer mon.Task()(&ctx)(&err)

	opts, err := tlsopts.NewOptions(identity, sc.Config)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, opts.RevDB.Close()) }()

	server, err := New(opts, sc.Address, sc.PrivateAddress, interceptor, services...)
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
