// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
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
	StatDB    *statdb.Server
	Overlay   *overlay.Cache

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

	node.Transport = transport.NewClient(identity)

	node.Provider, err = provider.NewProvider(node.Identity, node.Listener, grpcauth.NewAPIKeyInterceptor())
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
	if node.Provider != nil {
		errs = append(errs, node.Provider.Close())
	}
	// Provider automatically closes listener
	// if node.Listener != nil {
	//    errs = append(errs, node.Listener.Close())
	// }
	if node.Kademlia != nil {
		errs = append(errs, node.Kademlia.Disconnect())
	}

	for _, dep := range node.Dependencies {
		err := dep.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return utils.CombineErrors(errs...)
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
	routing, err := kademlia.NewRoutingTable(node.Info, teststore.New(), teststore.New())
	if err != nil {
		return err
	}

	kad, err := kademlia.NewKademliaWithRoutingTable(node.Info, planet.nodeInfos, node.Identity, 5, routing)
	if err != nil {
		return utils.CombineErrors(err, routing.Close())
	}

	node.Kademlia = kad

	node.Overlay = overlay.NewOverlayCache(teststore.New(), node.Kademlia, node.StatDB)

	return nil
}

// initStatDB creates statdb for a given planet
func (node *Node) initStatDB() error {
	dbPath := fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", rand.Int63())
	sdb, err := statdb.NewServer("sqlite3", dbPath, "", zap.NewNop())
	if err != nil {
		return err
	}
	node.StatDB = sdb
	return nil
}

type closerFunc func() error

func (fn closerFunc) Close() error { return fn() }
