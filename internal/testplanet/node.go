// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// Node is a general purpose
type Node struct {
	Log       *zap.Logger
	Info      pb.Node
	Identity  *provider.FullIdentity
	Transport transport.Client
}

// newUplink creates a new uplink
func (planet *Planet) newUplink(name string) (*Node, error) {
	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	node := &Node{
		Log:      planet.log.Named(name),
		Identity: identity,
	}

	node.Log.Debug("id=" + identity.ID.String())

	node.Transport = transport.NewClient(identity)

	node.Info = pb.Node{
		Id:   node.Identity.ID,
		Type: pb.NodeType_UPLINK,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   "",
		},
	}

	planet.nodes = append(planet.nodes, node)

	return node, nil
}

// ID returns node id
func (node *Node) ID() storj.NodeID { return node.Info.Id }

// Addr returns node address
func (node *Node) Addr() string { return node.Info.Address.Address }

// Local returns node info
func (node *Node) Local() pb.Node { return node.Info }

// Shutdown shuts down all node dependencies
func (node *Node) Shutdown() error { return nil }

// DialPointerDB dials destination with apikey and returns pointerdb Client
func (node *Node) DialPointerDB(destination Peer, apikey string) (pdbclient.Client, error) {
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
func (node *Node) DialOverlay(destination Peer) (overlay.Client, error) {
	info := destination.Local()
	conn, err := node.Transport.DialNode(context.Background(), &info, grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	// TODO: handle disconnect
	return overlay.NewClientFrom(pb.NewOverlayClient(conn)), nil
}

type closerFunc func() error

func (fn closerFunc) Close() error { return fn() }
