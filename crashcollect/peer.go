// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package crashcollect

import (
	"context"
	"errors"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/storj/crashcollect/crash"
	"storj.io/storj/private/crashreportpb"
	"storj.io/storj/private/server"
)

// Config is the global configuration for storj crash collect service.
type Config struct {
	Debug    debug.Config
	Server   server.Config
	Crash    crash.Config
	Identity identity.Config
}

// Peer is the representation of a storj crash collect service.
//
// architecture: Peer
type Peer struct {
	Log      *zap.Logger
	Config   Config
	Identity *identity.FullIdentity

	Server *server.Server
	Crash  struct {
		Service  *crash.Service
		Endpoint *crash.Endpoint
	}
}

// New is a constructor for storj crash collect Peer.
func New(log *zap.Logger, full *identity.FullIdentity, config Config) (peer *Peer, err error) {
	peer = &Peer{
		Log:      log,
		Config:   config,
		Identity: full,
	}

	peer.Crash.Service = crash.NewService(peer.Config.Crash)
	peer.Crash.Endpoint = crash.NewEndpoint(peer.Log, peer.Crash.Service)

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	tlsOptions, err := tlsopts.NewOptions(peer.Identity, tlsConfig, nil)
	if err != nil {
		return nil, err
	}

	peer.Server, err = server.New(log.Named("server"), tlsOptions, config.Server)
	if err != nil {
		return nil, errs.Combine(err, peer.Close())
	}

	err = crashreportpb.DRPCRegisterCrashReport(peer.Server.DRPC(), peer.Crash.Endpoint)
	if err != nil {
		return nil, err
	}

	peer.Log.Info("id = ", zap.Any("", full.ID.String()))

	return peer, nil
}

// Run runs storj crash collect Peer api until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	// start storj crash collect web api drpc server as a separate goroutine.
	group.Go(func() error {
		return ignoreCancel(peer.Server.Run(ctx))
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	if peer.Server != nil {
		return peer.Server.Close()
	}

	return nil
}

func ignoreCancel(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}
