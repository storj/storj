// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrap

import (
	"context"
	"net"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"storj.io/storj/bootstrap/bootstrapweb"
	"storj.io/storj/bootstrap/bootstrapweb/bootstrapserver"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
)

// DB is the master database for Boostrap Node
type DB interface {
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	// TODO: use better interfaces
	RoutingTable() (kdb, ndb storage.KeyValueStore)
}

// Config is all the configuration parameters for a Bootstrap Node
type Config struct {
	Identity identity.Config

	Server   server.Config
	Kademlia kademlia.Config

	Web bootstrapserver.Config
}

// Verify verifies whether configuration is consistent and acceptable.
func (config *Config) Verify(log *zap.Logger) error {
	return config.Kademlia.Verify(log)
}

// Peer is the representation of a Bootstrap Node.
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Transport transport.Client

	Server *server.Server

	// services and endpoints
	Kademlia struct {
		RoutingTable *kademlia.RoutingTable
		Service      *kademlia.Kademlia
		Endpoint     *kademlia.Endpoint
		Inspector    *kademlia.Inspector
	}

	// Web server with web UI
	Web struct {
		Listener net.Listener
		Service  *bootstrapweb.Service
		Endpoint *bootstrapserver.Server
	}
}

// New creates a new Bootstrap Node.
func New(log *zap.Logger, full *identity.FullIdentity, db DB, config Config) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	var err error

	{ // setup listener and server
		sc := config.Server
		options, err := tlsopts.NewOptions(peer.Identity, sc.Config)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Transport = transport.NewClient(options)

		peer.Server, err = server.New(options, sc.Address, sc.PrivateAddress, nil)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup kademlia
		config := config.Kademlia
		// TODO: move this setup logic into kademlia package
		if config.ExternalAddress == "" {
			config.ExternalAddress = peer.Addr()
		}

		self := pb.Node{
			Id:   peer.ID(),
			Type: pb.NodeType_BOOTSTRAP,
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   config.ExternalAddress,
			},
			Metadata: &pb.NodeMetadata{
				Email:  config.Operator.Email,
				Wallet: config.Operator.Wallet,
			},
		}

		kdb, ndb := peer.DB.RoutingTable()
		peer.Kademlia.RoutingTable, err = kademlia.NewRoutingTable(peer.Log.Named("routing"), self, kdb, ndb, &config.RoutingTableConfig)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Transport = peer.Transport.WithObservers(peer.Kademlia.RoutingTable)

		// TODO: reduce number of arguments
		peer.Kademlia.Service, err = kademlia.NewService(peer.Log.Named("kademlia"), self, config.BootstrapNodes(), peer.Transport, config.Alpha, peer.Kademlia.RoutingTable)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Kademlia.Endpoint = kademlia.NewEndpoint(peer.Log.Named("kademlia:endpoint"), peer.Kademlia.Service, peer.Kademlia.RoutingTable)
		pb.RegisterNodesServer(peer.Server.GRPC(), peer.Kademlia.Endpoint)

		peer.Kademlia.Inspector = kademlia.NewInspector(peer.Kademlia.Service, peer.Identity)
		pb.RegisterKadInspectorServer(peer.Server.PrivateGRPC(), peer.Kademlia.Inspector)
	}

	{ // setup bootstrap web ui
		config := config.Web

		peer.Web.Listener, err = net.Listen("tcp", config.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Web.Service, err = bootstrapweb.NewService(
			peer.Log.Named("bootstrapWeb:service"),
			peer.Kademlia.Service,
		)

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Web.Endpoint = bootstrapserver.NewServer(
			peer.Log.Named("bootstrapWeb:endpoint"),
			config,
			peer.Web.Service,
			peer.Web.Listener,
		)
	}

	return peer, nil
}

// Run runs bootstrap node until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return ignoreCancel(peer.Kademlia.Service.Bootstrap(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Kademlia.Service.Run(ctx))
	})
	group.Go(func() error {
		// TODO: move the message into Server instead
		// Don't change the format of this comment, it is used to figure out the node id.
		peer.Log.Sugar().Infof("Node %s started on %s", peer.Identity.ID, peer.Addr())
		peer.Log.Sugar().Infof("Node %s started on %s", peer.Identity.ID, peer.PrivateAddr())
		return ignoreCancel(peer.Server.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Web.Endpoint.Run(ctx))
	})

	return group.Wait()
}

func ignoreCancel(err error) error {
	if err == context.Canceled || err == grpc.ErrServerStopped || err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	// TODO: ensure that Close can be called on nil-s that way this code won't need the checks.

	// close servers, to avoid new connections to closing subsystems
	if peer.Server != nil {
		errlist.Add(peer.Server.Close())
	}

	if peer.Web.Endpoint != nil {
		errlist.Add(peer.Web.Endpoint.Close())
	} else {
		if peer.Web.Listener != nil {
			errlist.Add(peer.Web.Listener.Close())
		}
	}

	// close services in reverse initialization order
	if peer.Kademlia.Service != nil {
		errlist.Add(peer.Kademlia.Service.Close())
	}
	if peer.Kademlia.RoutingTable != nil {
		errlist.Add(peer.Kademlia.RoutingTable.Close())
	}

	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }

// Local returns the peer local node info.
func (peer *Peer) Local() pb.Node { return peer.Kademlia.RoutingTable.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Addr().String() }

// PrivateAddr returns the private address.
func (peer *Peer) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
