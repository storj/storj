// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// Node is the storj definition for a node in the network
type Node struct {
	dht   dht.DHT
	self  pb.Node
	cache *ConnectionPool
}

// Lookup queries nodes looking for a particular node in the network
func (n *Node) Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error) {
	c, err := n.cache.Dial(ctx, &to)
	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	resp, err := c.Query(ctx, &pb.QueryRequest{Limit: 20, Sender: &n.self, Target: &find, Pingback: true})
	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	rt, err := n.dht.GetRoutingTable(ctx)
	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	if err := rt.ConnectionSuccess(&to); err != nil {
		return nil, NodeClientErr.Wrap(err)

	}

	return resp.Response, nil
}

// Ping attempts to establish a connection with a node to verify it is alive
func (n *Node) Ping(ctx context.Context, to pb.Node) (bool, error) {
	_, err := n.cache.Dial(ctx, &to)
	if err != nil {
		return false, NodeClientErr.Wrap(err)
	}

	return true, nil
}

// Disconnect closes all connections within the cache
func (n *Node) Disconnect() error {
	return n.cache.DisconnectAll()
}
