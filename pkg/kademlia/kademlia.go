// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	// "crypto/rand"
	"fmt"
	// "log"
	"net"
	// "strconv"

	// bkad "github.com/coyle/kademlia"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/dht"
	proto "storj.io/storj/protos/overlay"
)

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
	options        KadOptions
	routingTable   dht.RoutingTable
	bootstrapNodes []proto.Node
	ip             string
	port           string
	stun           bool
	networking 	   networking
}

// NewKademlia returns a newly configured Kademlia instance
func NewKademlia(id dht.NodeID, bootstrapNodes []proto.Node, ip string, port string, kadOptions *KadOptions, routingOptions *RoutingOptions) (*Kademlia, error) {
	if port == "" {
		return nil, NodeErr.New("must specify port in request to NewKademlia")
	}

	ips, err := net.LookupIP(ip)
	if err != nil {
		return nil, err
	}

	if len(ips) <= 0 {
		return nil, errs.New("Invalid IP")
	}

	ip = ips[0].String()

	localNode = proto.Node{Id: id, Address: *NodeAddress} //TODO: what should node address be?
	rt := NewRoutingTable(localNode, routingOptions.kpath, routingOptions.npath, routingOptions.idLength, routingOptions.bucketSize)
	networking := networking{} //TODO
	
	return &Kademlia{
		options:		kadOptions,
		routingTable:   rt,
		bootstrapNodes: bootstrapNodes,
		ip:             ip,
		port:           port,
		stun:           true,
		networking:     networking,
	}, nil
}


// GetNodes returns all nodes from a starting node up to a maximum limit stored in the local routing table
func (k Kademlia) GetNodes(ctx context.Context, start string, limit int) ([]*proto.Node, error) {
	if start == "" {
		start = k.routingTable.Local()
	}
 
	nn, err := k.FindNodes() //TODO
	if err != nil {
		return []*proto.Node{}, err
	}
	return nn, nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k *Kademlia) GetRoutingTable(ctx context.Context) (dht.RoutingTable, error) {
	return k.routingTable, nil

}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k *Kademlia) Bootstrap(ctx context.Context) error {
	// return k.dht.Bootstrap()
	return
}

// Ping checks that the provided node is still accessible on the network
func (k *Kademlia) Ping(ctx context.Context, node proto.Node) (proto.Node, error) {
	// n, err := convertProtoNode(node)
	// if err != nil {
	// 	return proto.Node{}, err
	// }

	// ok, err := k.dht.Ping(n)
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
func (k *Kademlia) FindNode(ctx context.Context, ID dht.NodeID) (proto.Node, error) {
	// nodes, err := k.dht.FindNode(ID.Bytes())
	// if err != nil {
	// 	return proto.Node{}, err

	// }

	for _, v := range nodes {
		if string(v.ID) == ID.String() {
			return proto.Node{Id: string(v.ID), Address: &proto.NodeAddress{
				Transport: defaultTransport,
				Address:   fmt.Sprintf("%s:%d", v.IP.String(), v.Port),
			},
			}, nil
		}
	}
	return proto.Node{}, NodeErr.New("node not found")
}

// Disconnect safely closes connections to the Kademlia network
func (k Kademlia) Disconnect() error {
	return k.networking.Disconnect()
}
