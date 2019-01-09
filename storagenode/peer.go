// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// DB is the master database for Storage Node
type DB interface {
	// TODO: use better interfaces
	Disk() string
	PSDB() *psdb.DB
	RoutingTable() (kdb, ndb storage.KeyValueStore)
}

// Config is all the configuration parameters for a Storage Node
type Config struct {
	Identity identity.Config

	// TODO: switch to using server.Config when Identity has been removed from it
	PublicAddress string `help:"public address to listen on" default:":7777"`
	Kademlia      kademlia.Config
	Piecestore    psserver.Config
}

// Verify verifies whether configuration is consistent and acceptable.
func (config *Config) Verify() error {
	return nil
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
		peer.Public.Listener, err = net.Listen("tcp", config.PublicAddress)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		publicConfig := provider.ServerConfig{Address: peer.Public.Listener.Addr().String()}
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
		config := config.Piecestore

		// TODO: psserver shouldn't need the private key
		peer.Piecestore = psserver.New(peer.Log.Named("piecestore"), peer.DB.Disk(), peer.DB.PSDB(), config, peer.Identity.Key)
		pb.RegisterPieceStoreRoutesServer(peer.Public.Server.GRPC(), peer.Piecestore)
	}

	return peer, nil
}

// Run runs storage node until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := peer.Kademlia.Bootstrap(ctx); err != nil {
		return err
	}
	peer.Kademlia.StartRefresh(ctx)

	return peer.Public.Server.Run(ctx)
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
	}
	if peer.Public.Listener != nil {
		errlist.Add(peer.Public.Listener.Close())
	}
	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }

// Local returns the peer local node info.
func (peer *Peer) Local() pb.Node { return peer.RoutingTable.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Public.Server.Addr().String() }
