// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"fmt"
	"strings"

	bkad "github.com/coyle/kademlia"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	proto "storj.io/storj/protos/overlay"
)

// NodeErr is the class for all errros petaining to node operations
var NodeErr = errs.Class("node error")
var defaultTransport = "udp"

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
	return []proto.Node{}, nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k Kademlia) GetRoutingTable(ctx context.Context) (RoutingTable, error) {
	fmt.Println("pkg/kademlia/dht.go -- routing table", k.rt)
	return k.rt, nil
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

	ok, err := k.dht.Ping(convert(node))
	if err != nil {
		return proto.Node{}, err
	}
	if !ok {
		return proto.Node{}, NodeErr.New("node unavailable")
	}
	return node, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k Kademlia) FindNode(ctx context.Context, ID NodeID) (proto.Node, error) {
	nodes, err := k.dht.FindNode([]byte(ID))
	fmt.Println(nodes)

	if err != nil {
		return proto.Node{}, err

	}

	if len(nodes) <= 0 || string(nodes[0].ID) != string(ID) {
		// check if the IDs don't match since dht.FindNode will
		// return the closest node if the node it's looking for
		// is not found
		return proto.Node{}, NodeErr.New("node not found")
	}

	node := nodes[0]

	return proto.Node{
		Id: string(node.ID),
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP, // TODO: defaulting to this, probably needs to be determined during lookup
			Address:   fmt.Sprintf("%s:%d", node.IP.String(), node.Port),
		},
	}, nil
}

func convertNodeTypes(n []proto.Node) []*bkad.NetworkNode {
	nn := []*bkad.NetworkNode{}
	for i, v := range n {
		nn[i] = convert(v)
	}

	return nn
}

func convert(n proto.Node) *bkad.NetworkNode {
	ip := strings.Split(n.GetAddress().GetAddress(), ":")
	if len(ip) == 1 {
		ip = append(ip, "0")
	}

	nn := bkad.NewNetworkNode(ip[0], ip[1])
	nn.ID = []byte(n.GetId())

	return nn
}

func GetNodeRoutingTable(ctx context.Context, ID NodeID) (RouteTable, error) {
	return RouteTable{}, nil
}
