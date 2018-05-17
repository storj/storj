// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"storj.io/storj/protos/overlay"
)

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
}

// GetNodes returns all nodes from a starting node up to a maximum limit
func (k Kademlia) GetNodes(ctx context.Context, start string, limit int) ([]overlay.Node, error) {
	return []overlay.Node{}, nil
}

// GetRoutingTable provides the routing table for the Kademlia DHT
func (k Kademlia) GetRoutingTable(ctx context.Context) (RoutingTable, error) {
	return RouteTable{}, nil
}

// Bootstrap contacts one of a set of pre defined trusted nodes on the network and
// begins populating the local Kademlia node
func (k Kademlia) Bootstrap(ctx context.Context) error {
	return nil
}

// Ping checks that the provided node is still accessible on the network
func (k Kademlia) Ping(ctx context.Context, node overlay.Node) (overlay.Node, error) {
	return overlay.Node{}, nil
}

// FindNode looks up the provided NodeID first in the local Node, and if it is not found
// begins searching the network for the NodeID. Returns and error if node was not found
func (k Kademlia) FindNode(ctx context.Context, ID NodeID) (overlay.Node, error) {
	return overlay.Node{}, nil
}
