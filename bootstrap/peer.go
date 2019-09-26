// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrap

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/bootstrap/bootstrapweb"
	"storj.io/storj/bootstrap/bootstrapweb/bootstrapserver"
	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
)

// DB is the master database for Boostrap Node
type DB interface {
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	// TODO: use better interfaces
	RoutingTable() (kdb, ndb, adb storage.KeyValueStore)
}

// Config is all the configuration parameters for a Bootstrap Node
type Config struct {
	Identity identity.Config

	Server   server.Config
	Kademlia kademlia.Config

	Web bootstrapserver.Config

	Version version.Config
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

	Dialer rpc.Dialer

	Server *server.Server

	Version *version.Service

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
func New(log *zap.Logger, full *identity.FullIdentity, db DB, revDB extensions.RevocationDB, config Config, versionInfo version.Info) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	var err error

	{
		test := version.Info{}
		if test != versionInfo {
			peer.Log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
				versionInfo.Version.String(), versionInfo.CommitHash, versionInfo.Timestamp.String(), versionInfo.Release)
		}
		peer.Version = version.NewService(log.Named("version"), config.Version, versionInfo, "Bootstrap")
	}

	{ // setup listener and server
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Dialer = rpc.NewDefaultDialer(tlsOptions)

		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress, nil)
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

		pbVersion, err := versionInfo.Proto()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		self := &overlay.NodeDossier{
			Node: pb.Node{
				Id: peer.ID(),
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   config.ExternalAddress,
				},
			},
			Type: pb.NodeType_BOOTSTRAP,
			Operator: pb.NodeOperator{
				Email:  config.Operator.Email,
				Wallet: config.Operator.Wallet,
			},
			Version: *pbVersion,
		}

		kdb, ndb, adb := peer.DB.RoutingTable()
		peer.Kademlia.RoutingTable, err = kademlia.NewRoutingTable(peer.Log.Named("routing"), self, kdb, ndb, adb, &config.RoutingTableConfig)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Kademlia.Service, err = kademlia.NewService(peer.Log.Named("kademlia"), peer.Dialer, peer.Kademlia.RoutingTable, config)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Kademlia.Endpoint = kademlia.NewEndpoint(peer.Log.Named("kademlia:endpoint"), peer.Kademlia.Service, nil, peer.Kademlia.RoutingTable, nil)
		pb.RegisterNodesServer(peer.Server.GRPC(), peer.Kademlia.Endpoint)
		pb.DRPCRegisterNodes(peer.Server.DRPC(), peer.Kademlia.Endpoint)

		peer.Kademlia.Inspector = kademlia.NewInspector(peer.Kademlia.Service, peer.Identity)
		pb.RegisterKadInspectorServer(peer.Server.PrivateGRPC(), peer.Kademlia.Inspector)
		pb.DRPCRegisterKadInspector(peer.Server.PrivateDRPC(), peer.Kademlia.Inspector)
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
		return errs2.IgnoreCanceled(peer.Version.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Kademlia.Service.Bootstrap(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Kademlia.Service.Run(ctx))
	})
	group.Go(func() error {
		// TODO: move the message into Server instead
		// Don't change the format of this comment, it is used to figure out the node id.
		peer.Log.Sugar().Infof("Node %s started", peer.Identity.ID)
		peer.Log.Sugar().Infof("Public server started on %s", peer.Addr())
		peer.Log.Sugar().Infof("Private server started on %s", peer.PrivateAddr())
		return errs2.IgnoreCanceled(peer.Server.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Web.Endpoint.Run(ctx))
	})

	return group.Wait()
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
	} else if peer.Web.Listener != nil {
		errlist.Add(peer.Web.Listener.Close())
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
func (peer *Peer) Local() overlay.NodeDossier { return peer.Kademlia.RoutingTable.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Addr().String() }

// URL returns the storj.NodeURL
func (peer *Peer) URL() storj.NodeURL { return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()} }

// PrivateAddr returns the private address.
func (peer *Peer) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
