// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"strings"

	bkad "github.com/coyle/kademlia"
	"golang.org/x/sync/errgroup"

	proto "storj.io/storj/protos/overlay"
)

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	rt             RoutingTable
	bootstrapNodes []proto.Node
	ip             string
	port           string
	stun           bool
	dht            *bkad.DHT
}

// GetNodes returns all nodes from a starting node up to a maximum limit
func (k Kademlia) GetNodes(ctx context.Context, start string, limit int) ([]proto.Node, error) {
	// k.dht.Get
	return []proto.Node{}, nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k Kademlia) GetRoutingTable(ctx context.Context) (RoutingTable, error) {
	return RouteTable{}, nil
}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k Kademlia) Bootstrap(ctx context.Context) error {

	dht, err := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
		BootstrapNodes: convertNodeTypes(k.bootstrapNodes),
		IP:             k.ip,
		Port:           k.port,
		UseStun:        k.stun,
	})
	if err != nil {
		return err
	}

	if err := dht.CreateSocket(); err != nil {
		return err
	}

	g := errgroup.Group{}

	g.Go(func() error {
		return dht.Listen()
	})

	g.Go(func() error {
		return dht.Bootstrap()
	})

	return g.Wait()
}

// Ping checks that the provided node is still accessible on the network
func (k Kademlia) Ping(ctx context.Context, node proto.Node) (proto.Node, error) {
	return proto.Node{}, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k Kademlia) FindNode(ctx context.Context, ID NodeID) (proto.Node, error) {
	nodes, err := k.dht.FindNode([]byte(ID))
	if err != nil {
		return proto.Node{}, err

	}
	return proto.Node{}, nil
}

func convertNodeTypes(n []proto.Node) []*bkad.NetworkNode {
	nn := []*bkad.NetworkNode{}
	for i, v := range n {
		ip := strings.Split(v.GetAddress().GetAddress(), ":")
		if len(ip) < 2 {

		}

		n := bkad.NewNetworkNode(ip[0], ip[1])
		n.ID = []byte(v.GetId())
		nn[i] = n
	}

	return nn
}
