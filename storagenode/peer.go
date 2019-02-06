// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/agreementsender"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
)

// DB is the master database for Storage Node
type DB interface {
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	// TODO: use better interfaces
	Storage() *pstore.Storage
	PSDB() *psdb.DB
	RoutingTable() (kdb, ndb storage.KeyValueStore)
}

// Config is all the configuration parameters for a Storage Node
type Config struct {
	Identity identity.Config

	Server   server.Config
	Kademlia kademlia.Config
	Storage  psserver.Config
}

// Verify verifies whether configuration is consistent and acceptable.
func (config *Config) Verify(log *zap.Logger) error {
	return config.Kademlia.Verify(log)
}

// Peer is the representation of a Storage Node.
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Transport transport.Client

	// servers
	Public struct {
		Listener net.Listener
		Server   *server.Server
	}

	// services and endpoints
	// TODO: similar grouping to satellite.Peer
	Kademlia struct {
		RoutingTable *kademlia.RoutingTable
		Service      *kademlia.Kademlia
		Endpoint     *kademlia.Endpoint
		Inspector    *kademlia.Inspector
	}

	Storage struct {
		Endpoint  *psserver.Server // TODO: separate into endpoint and service
		Monitor   *psserver.Monitor
		Collector *psserver.Collector
	}

	Agreements struct {
		Sender *agreementsender.AgreementSender
	}
}

// New creates a new Storage Node.
func New(log *zap.Logger, full *identity.FullIdentity, db DB, config Config) (*Peer, error) {
	peer := &Peer{
		Log:       log,
		Identity:  full,
		DB:        db,
		Transport: transport.NewClient(full),
	}

	var err error

	{ // setup listener and server
		peer.Public.Listener, err = net.Listen("tcp", config.Server.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		publicConfig := server.Config{Address: peer.Public.Listener.Addr().String()}
		publicOptions, err := server.NewOptions(peer.Identity, publicConfig)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Public.Server, err = server.New(publicOptions, peer.Public.Listener, nil)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup kademlia
		config := config.Kademlia
		// TODO: move this setup logic into kademlia package
		if config.ExternalAddress == "" {
			config.ExternalAddress = peer.Public.Server.Addr().String()
		}

		self := pb.Node{
			Id:   peer.ID(),
			Type: pb.NodeType_STORAGE,
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

		// TODO: reduce number of arguments
		peer.Kademlia.Service, err = kademlia.NewService(peer.Log.Named("kademlia"), self, config.BootstrapNodes(), peer.Identity, config.Alpha, peer.Kademlia.RoutingTable)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Kademlia.Endpoint = kademlia.NewEndpoint(peer.Log.Named("kademlia:endpoint"), peer.Kademlia.Service, peer.Kademlia.RoutingTable)
		pb.RegisterNodesServer(peer.Public.Server.GRPC(), peer.Kademlia.Endpoint)

		peer.Kademlia.Inspector = kademlia.NewInspector(peer.Kademlia.Service, peer.Identity)
		pb.RegisterKadInspectorServer(peer.Public.Server.GRPC(), peer.Kademlia.Inspector)
	}

	{ // setup piecestore
		// TODO: move this setup logic into psstore package
		config := config.Storage

		// TODO: psserver shouldn't need the private key
		peer.Storage.Endpoint, err = psserver.NewEndpoint(peer.Log.Named("piecestore"), config, peer.DB.Storage(), peer.DB.PSDB(), peer.Identity.Key, peer.Kademlia.Service)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		pb.RegisterPieceStoreRoutesServer(peer.Public.Server.GRPC(), peer.Storage.Endpoint)

		// TODO: organize better
		peer.Storage.Monitor = psserver.NewMonitor(peer.Log.Named("piecestore:monitor"), config.KBucketRefreshInterval, peer.Kademlia.RoutingTable, peer.Storage.Endpoint)
		peer.Storage.Collector = psserver.NewCollector(peer.Log.Named("piecestore:collector"), peer.DB.PSDB(), peer.DB.Storage(), config.CollectorInterval)
	}

	{ // agreements
		config := config.Storage // TODO: separate config
		peer.Agreements.Sender = agreementsender.New(
			peer.Log.Named("agreements"),
			peer.DB.PSDB(), peer.Identity, peer.Kademlia.Service,
			config.AgreementSenderCheckInterval,
		)
	}

	return peer, nil
}

// Run runs storage node until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return ignoreCancel(peer.Kademlia.Service.Bootstrap(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Kademlia.Service.RunRefresh(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Agreements.Sender.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Storage.Monitor.Run(ctx))
	})
	group.Go(func() error {
		return ignoreCancel(peer.Storage.Collector.Run(ctx))
	})
	group.Go(func() error {
		// TODO: move the message into Server instead
		peer.Log.Sugar().Infof("Node %s started on %s", peer.Identity.ID, peer.Public.Server.Addr().String())
		return ignoreCancel(peer.Public.Server.Run(ctx))
	})

	return group.Wait()
}

func ignoreCancel(err error) error {
	if err == context.Canceled || err == grpc.ErrServerStopped {
		return nil
	}
	return err
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	// TODO: ensure that Close can be called on nil-s that way this code won't need the checks.

	// close servers, to avoid new connections to closing subsystems
	if peer.Public.Server != nil {
		errlist.Add(peer.Public.Server.Close())
	} else {
		// peer.Public.Server automatically closes listener
		if peer.Public.Listener != nil {
			errlist.Add(peer.Public.Listener.Close())
		}
	}

	// close services in reverse initialization order
	if peer.Storage.Endpoint != nil {
		errlist.Add(peer.Storage.Endpoint.Close())
	}
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
func (peer *Peer) Addr() string { return peer.Public.Server.Addr().String() }
