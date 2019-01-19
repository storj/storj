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
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// DB is the master database for Storage Node
type DB interface {
	// TODO: use better interfaces
	Storage() *pstore.Storage
	PSDB() *psdb.DB
	RoutingTable() (kdb, ndb storage.KeyValueStore)
}

// Config is all the configuration parameters for a Storage Node
type Config struct {
	Server   server.Config
	Kademlia kademlia.Config
	Storage  psserver.Config
}

// Verify verifies whether configuration is consistent and acceptable.
func (config *Config) Verify() error {
	return config.Kademlia.Verify()
}

// Peer is the representation of a Storage Node.
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	// servers
	Public struct {
		Listener net.Listener
		Server   *server.Server
	}

	// services and endpoints
	RoutingTable     *kademlia.RoutingTable
	Kademlia         *kademlia.Kademlia
	KademliaEndpoint *node.Server

	Piecestore *psserver.Server // TODO: separate into endpoint and service
}

// New creates a new Storage Node.
func New(log *zap.Logger, full *identity.FullIdentity, db DB, config Config) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
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

		peer.Public.Server, err = server.NewServer(publicOptions, peer.Public.Listener, nil)
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
				Address: config.ExternalAddress,
			},
			Metadata: &pb.NodeMetadata{
				Email:  config.Operator.Email,
				Wallet: config.Operator.Wallet,
			},
		}

		kdb, ndb := peer.DB.RoutingTable()
		peer.RoutingTable, err = kademlia.NewRoutingTable(peer.Log.Named("routing"), self, kdb, ndb)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		// TODO: reduce number of arguments
		peer.Kademlia, err = kademlia.NewWith(peer.Log.Named("kademlia"), self, nil, peer.Identity, config.Alpha, peer.RoutingTable)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.KademliaEndpoint = node.NewServer(peer.Log.Named("kademlia:endpoint"), peer.Kademlia)
		pb.RegisterNodesServer(peer.Public.Server.GRPC(), peer.KademliaEndpoint)
	}

	{ // setup piecestore
		// TODO: move this setup logic into psstore package
		config := config.Storage

		// TODO: psserver shouldn't need the private key
		peer.Piecestore = psserver.New(peer.Log.Named("piecestore"), peer.DB.Storage(), peer.DB.PSDB(), config, peer.Identity.Key)
		pb.RegisterPieceStoreRoutesServer(peer.Public.Server.GRPC(), peer.Piecestore)
	}

	return peer, nil
}

// Run runs storage node until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var group errgroup.Group
	group.Go(func() error {
		err := peer.Kademlia.Bootstrap(ctx)
		if ctx.Err() == context.Canceled {
			// ignore err when when bootstrap was canceled
			return nil
		}
		return err
	})
	group.Go(func() error {
		err := peer.Kademlia.RunRefresh(ctx)
		if err == context.Canceled || err == grpc.ErrServerStopped {
			err = nil
		}
		return err
	})
	group.Go(func() error {
		err := peer.Public.Server.Run(ctx)
		if err == context.Canceled || err == grpc.ErrServerStopped {
			err = nil
		}
		return err
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	// TODO: ensure that Close can be called on nil-s that way this code won't need the checks.

	// close services in reverse initialization order
	if peer.Piecestore != nil {
		errlist.Add(peer.Piecestore.Close())
	}
	if peer.Kademlia != nil {
		errlist.Add(peer.Kademlia.Close())
	}
	if peer.RoutingTable != nil {
		errlist.Add(peer.RoutingTable.SelfClose())
	}

	// close servers
	if peer.Public.Server != nil {
		errlist.Add(peer.Public.Server.Close())
	} else {
		// peer.Public.Server automatically closes listener
		if peer.Public.Listener != nil {
			errlist.Add(peer.Public.Listener.Close())
		}
	}
	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }

// Local returns the peer local node info.
func (peer *Peer) Local() pb.Node { return peer.RoutingTable.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Public.Server.Addr().String() }
