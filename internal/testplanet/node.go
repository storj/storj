// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"io"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/discovery"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storage/teststore"
)

// Node is a general purpose
type Node struct {
	Log       *zap.Logger
	Info      pb.Node
	Identity  *provider.FullIdentity
	Transport transport.Client
	Listener  net.Listener
	Provider  *provider.Provider
	Kademlia  *kademlia.Kademlia
	Discovery *discovery.Discovery
	StatDB    statdb.DB
	Overlay   *overlay.Cache
	Database  *satellitedb.DB

	Dependencies []io.Closer
}

// newNode creates a new node.
func (planet *Planet) newNode(name string, nodeType pb.NodeType) (*Node, error) {
	identity, err := planet.newIdentity()
	if err != nil {
		return nil, err
	}

	listener, err := planet.newListener()
	if err != nil {
		return nil, err
	}

	node := &Node{
		Log:      planet.log.Named(name),
		Identity: identity,
		Listener: listener,
	}

	node.Log.Debug("id=" + identity.ID.String())

	node.Transport = transport.NewClient(identity)

	serverConfig := provider.ServerConfig{Address: node.Listener.Addr().String()}
	opts, err := provider.NewServerOptions(node.Identity, serverConfig)
	if err != nil {
		return nil, err
	}
	node.Provider, err = provider.NewProvider(opts, node.Listener, grpcauth.NewAPIKeyInterceptor())
	if err != nil {
		return nil, utils.CombineErrors(err, listener.Close())
	}

	node.Info = pb.Node{
		Id:   node.Identity.ID,
		Type: nodeType,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   node.Listener.Addr().String(),
		},
	}

	planet.nodes = append(planet.nodes, node)
	planet.nodeInfos = append(planet.nodeInfos, node.Info)
	planet.nodeLinks = append(planet.nodeLinks, node.Info.Id.String()+":"+node.Listener.Addr().String())

	return node, nil
}

// ID returns node id
func (node *Node) ID() storj.NodeID { return node.Info.Id }

// Addr retursn node address
func (node *Node) Addr() string { return node.Info.Address.Address }

// Shutdown shuts down all node dependencies
func (node *Node) Shutdown() error {
	var errs []error
	if node.Kademlia != nil {
		errs = append(errs, node.Kademlia.Disconnect())
	}
	if node.Provider != nil {
		errs = append(errs, node.Provider.Close())
	}
	// Provider automatically closes listener
	// if node.Listener != nil {
	//    errs = append(errs, node.Listener.Close())
	// }

	for _, dep := range node.Dependencies {
		err := dep.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return utils.CombineErrors(errs...)
}

// NewNodeClient creates a node client for this node
func (n *Node) NewNodeClient() (node.Client, error) { //nolint renaming to node would conflict with package name; rename Node to Peer to resolve
	// TODO: handle disconnect verification
	return node.NewNodeClient(n.Identity, n.Info, n.Kademlia)
}

// DialPointerDB dials destination with apikey and returns pointerdb Client
func (node *Node) DialPointerDB(destination *Node, apikey string) (pdbclient.Client, error) {
	// TODO: use node.Transport instead of pdbclient.NewClient
	/*
		conn, err := node.Transport.DialNode(context.Background(), &destination.Info)
		if err != nil {
			return nil, err
		}
		return piececlient.NewPSClient
	*/

	// TODO: handle disconnect
	return pdbclient.NewClient(node.Identity, destination.Addr(), apikey)
}

// DialOverlay dials destination and returns an overlay.Client
func (node *Node) DialOverlay(destination *Node) (overlay.Client, error) {
	conn, err := node.Transport.DialNode(context.Background(), &destination.Info, grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	// TODO: handle disconnect
	return overlay.NewClientFrom(pb.NewOverlayClient(conn)), nil
}

// initOverlay creates overlay for a given planet
func (node *Node) initOverlay(planet *Planet) error {
	var err error
	node.Database, err = satellitedb.NewInMemory()
	if err != nil {
		return err
	}

	err = node.Database.CreateTables()
	if err != nil {
		return err
	}

	routing, err := kademlia.NewRoutingTable(node.Info, teststore.New(), teststore.New())
	if err != nil {
		return err
	}

	kad, err := kademlia.NewKademliaWithRoutingTable(node.Log.Named("kademlia"), node.Info, planet.nodeInfos, node.Identity, 5, routing)
	if err != nil {
		return utils.CombineErrors(err, routing.Close())
	}
	node.Kademlia = kad

	node.StatDB = node.Database.StatDB()

	node.Overlay = overlay.NewOverlayCache(teststore.New(), node.Kademlia, node.StatDB)
	node.Discovery = discovery.NewDiscovery(node.Overlay, node.Kademlia, node.StatDB, zap.L())

	return nil
}

type closerFunc func() error

func (fn closerFunc) Close() error { return fn() }
