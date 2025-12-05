// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"errors"
	"net"
	"runtime/pprof"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/debug"
	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleauth/csrf"
	"storj.io/storj/satellite/console/consoleweb"
)

// UI is the satellite UI process.
//
// architecture: Peer
type UI struct {
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Servers *lifecycle.Group

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Console struct {
		Listener net.Listener
		Server   *consoleweb.Server
	}

	CSRF struct {
		Service *csrf.Service
	}
}

// NewUI creates a new satellite UI process.
func NewUI(log *zap.Logger, full *identity.FullIdentity, config *Config, atomicLogLevel *zap.AtomicLevel, satelliteAddr, consoleBackendAddr string) (*UI, error) {
	peer := &UI{
		Log:      log,
		Identity: full,

		Servers: lifecycle.NewGroup(log.Named("servers")),
	}

	{ // setup debug
		var err error
		if config.Debug.Addr != "" {
			peer.Debug.Listener, err = net.Listen("tcp", config.Debug.Addr)
			if err != nil {
				withoutStack := errors.New(err.Error())
				peer.Log.Debug("failed to start debug endpoints", zap.Error(withoutStack))
			}
		}
		debugConfig := config.Debug
		debugConfig.ControlTitle = "UI"
		peer.Debug.Server = debug.NewServerWithAtomicLevel(log.Named("debug"), peer.Debug.Listener, monkit.Default, debugConfig, atomicLogLevel)
		peer.Servers.Add(lifecycle.Item{
			Name:  "debug",
			Run:   peer.Debug.Server.Run,
			Close: peer.Debug.Server.Close,
		})
	}

	var err error

	{ // setup console
		consoleConfig := config.Console

		if consoleConfig.AuthTokenSecret == "" {
			return nil, errs.New("Auth token secret required")
		}

		signer := &consoleauth.Hmac{Secret: []byte(consoleConfig.AuthTokenSecret)}
		peer.CSRF.Service = csrf.NewService(signer)

		peer.Console.Listener, err = net.Listen("tcp", consoleConfig.FrontendAddress)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Server, err = consoleweb.NewFrontendServer(
			peer.Log.Named("console:endpoint"),
			consoleConfig,
			peer.Console.Listener,
			storj.NodeURL{ID: peer.ID(), Address: satelliteAddr},
			peer.CSRF.Service,
			config.Payments.StripeCoinPayments.StripePublicKey,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Servers.Add(lifecycle.Item{
			Name:  "console:endpoint",
			Run:   peer.Console.Server.RunFrontend,
			Close: peer.Console.Server.Close,
		})
	}

	return peer, nil
}

// Run runs satellite UI until it's either closed or it errors.
func (peer *UI) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "ui"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *UI) Close() error {
	return peer.Servers.Close()
}

// ID returns the peer ID.
func (peer *UI) ID() storj.NodeID { return peer.Identity.ID }
