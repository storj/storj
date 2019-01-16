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
	RevocationDBURL     string `help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6378?db=2&password=abc123)" default:"bolt://$CONFDIR/revocations.db"`
	PeerCAWhitelistPath string `help:"path to the CA cert whitelist (peer identities must be signed by one these to be verified)"`
	Address             string `user:"true" help:"address to listen on" default:":7777"`
	Extensions          peertls.TLSExtConfig

	Identity identity.Config
}

// Run will run the given responsibilities with the configured identity.
func (sc Config) Run(ctx context.Context,
	interceptor grpc.UnaryServerInterceptor, services ...Service) (err error) {
	defer mon.Task()(&ctx)(&err)

	ident, err := sc.Identity.Load()
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", sc.Address)
	if err != nil {
		return err
	}
	defer func() { _ = lis.Close() }()

	opts, err := NewOptions(ident, sc)
	if err != nil {
		return err
	}
	defer func() { err = utils.CombineErrors(err, opts.RevDB.Close()) }()

	s, err := NewServer(opts, lis, interceptor, services...)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	zap.S().Infof("Node %s started on '%s'", s.Identity().ID, sc.Address)
	return s.Run(ctx)
}
