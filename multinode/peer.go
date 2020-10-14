// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"
	"net"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity"
	"storj.io/storj/multinode/console"
	"storj.io/storj/multinode/console/server"
	"storj.io/storj/private/lifecycle"
)

var (
	mon = monkit.Package()
)

// DB is the master database for Multinode Dashboard.
//
// architecture: Master Database
type DB interface {
	// Nodes returns nodes database.
	Nodes() console.Nodes
	// Members returns members database.
	Members() console.Members

	// Close closes the database.
	Close() error
	// CreateSchema creates schema.
	CreateSchema(ctx context.Context) error
}

// Config is all the configuration parameters for a Multinode Dashboard.
type Config struct {
	Identity identity.Config
	Console  server.Config
}

// Peer is the a Multinode Dashboard application itself.
//
// architecture: Peer
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	// Web server with web UI
	Console struct {
		Listener net.Listener
		// TODO: Service  *console.Service
		Endpoint *server.Server
	}

	Servers *lifecycle.Group
}

// New creates a new instance of Multinode Dashboard application.
func New(log *zap.Logger, full *identity.FullIdentity, config Config, db DB) (_ *Peer, err error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	{ // console setup
		// peer.Console.Service = console.NewService(
		// 	 peer.Log.Named("console:service"),
		// )

		peer.Console.Listener, err = net.Listen("tcp", config.Console.Address)
		if err != nil {
			return nil, err
		}

		peer.Console.Endpoint, err = server.NewServer(
			peer.Log.Named("console:endpoint"),
			config.Console,
			peer.Console.Listener,
		)
		if err != nil {
			return nil, err
		}
		peer.Servers.Add(lifecycle.Item{
			Name:  "console:endpoint",
			Run:   peer.Console.Endpoint.Run,
			Close: peer.Console.Endpoint.Close,
		})
	}

	return peer, nil
}

// Run runs Multinode Dashboard services and servers until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	peer.Servers.Run(ctx, group)

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	return peer.Servers.Close()
}
