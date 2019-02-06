// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

// Config holds server specific configuration parameters
type Config struct {
	RevocationDBURL     string `default:"bolt://$CONFDIR/revocations.db" help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6378?db=2&password=abc123)"`
	PeerCAWhitelistPath string `help:"path to the CA cert whitelist (peer identities must be signed by one these to be verified). this will override the default peer whitelist"`
	UsePeerCAWhitelist  bool   `help:"if true, uses peer ca whitelist checking" default:"false"`
	Address             string `user:"true" help:"address to listen on" default:":7777"`
	Extensions          peertls.TLSExtConfig
}

// Run will run the given responsibilities with the configured identity.
func (sc Config) Run(ctx context.Context, identity *identity.FullIdentity, interceptor grpc.UnaryServerInterceptor, services ...Service) (err error) {
	defer mon.Task()(&ctx)(&err)

	lis, err := net.Listen("tcp", sc.Address)
	if err != nil {
		return err
	}
	defer func() { _ = lis.Close() }()

	opts, err := NewOptions(identity, sc)
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
