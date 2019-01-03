// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"io"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/discovery"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/storage/teststore"
)

// Node is a general purpose
type Node struct {
	Log             *zap.Logger
	Info            pb.Node
	Identity        *identity.FullIdentity
	Transport       transport.Client
	PublicListener  net.Listener
	PrivateListener net.Listener
	Server          *server.Server
	Kademlia        *kademlia.Kademlia
	Discovery       *discovery.Discovery
	StatDB          statdb.DB
	Overlay         *overlay.Cache
	Database        satellite.DB

	Dependencies []io.Closer
}

// newNode creates a new node.
func (planet *Planet) newNode(name string, nodeType pb.NodeType) (*Node, error) {
	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	publicListener, err := planet.newListener()
	if err != nil {
		return nil, err
	}

	privateListener, err := planet.newListener()
	if err != nil {
		return nil, utils.CombineErrors(err, publicListener.Close())
	}

	node := &Node{
		Log:             planet.log.Named(name),
		Identity:        identity,
		PublicListener:  publicListener,
		PrivateListener: privateListener,
	}

	node.Log.Debug("id=" + identity.ID.String())

	node.Transport = transport.NewClient(identity)

	serverConfig := server.Config{}
	cfgstruct.SetStructDefaults(&serverConfig)
	serverConfig.PublicAddress = node.PublicListener.Addr().String()
	serverConfig.PrivateAddress = node.PrivateListener.Addr().String()

	// TODO(jt): should testplanet create a shared revocation db,
	// a per-node revocation db, or no revocation db?
	// should it create a ca whitelist? if we do create a per-node revocation db,
	// it should be added to node.Dependencies?
	pcvs := server.PCVs(nil, nil, serverConfig.CertVerification.Extensions)

	publicSrv, privateSrv, err := server.SetupRPCs(node.Log, identity, pcvs)
	if err != nil {
		return nil, utils.CombineErrors(err, publicListener.Close(), privateListener.Close())
	}

	node.Server = server.NewServer(
		identity,
		server.NewHandle(publicSrv, publicListener),
		server.NewHandle(privateSrv, privateListener))

	node.Info = pb.Node{
		Id:   node.Identity.ID,
		Type: nodeType,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   node.PublicListener.Addr().String(),
		},
	}

	planet.nodes = append(planet.nodes, node)
	planet.nodeInfos = append(planet.nodeInfos, node.Info)
	planet.nodeLinks = append(planet.nodeLinks, node.Info.Id.String()+":"+node.PublicListener.Addr().String())

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
	if node.Server != nil {
		errs = append(errs, node.Server.Close())
	}
	// Server automatically closes listeners

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

	routing, err := kademlia.NewRoutingTable(node.Log.Named("routing"), node.Info, teststore.New(), teststore.New())
	if err != nil {
		return err
	}

	kad, err := kademlia.NewKademliaWithRoutingTable(node.Log.Named("kademlia"), node.Info, planet.nodeInfos, node.Identity, 5, routing)
	if err != nil {
		return utils.CombineErrors(err, routing.Close())
	}

	node.Kademlia = kad
	node.StatDB = node.Database.StatDB()
	node.Overlay = overlay.NewCache(teststore.New(), node.StatDB)
	node.Discovery = discovery.NewDiscovery(node.Log.Named("discovery"), node.Overlay, node.Kademlia, node.StatDB)

	return nil
}

type closerFunc func() error

func (fn closerFunc) Close() error { return fn() }
