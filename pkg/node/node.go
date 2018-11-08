// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"
	"fmt"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// Node is the storj definition for a node in the network
type Node struct {
	dht  dht.DHT
	self pb.Node
	pool *ConnectionPool
}

// Lookup queries nodes looking for a particular node in the network
func (n *Node) Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error) {
	fmt.Printf("ATTEMPTING TO DIAL :: %s\n", to.GetAddress().Address)
	c, err := n.pool.Dial(ctx, &to)
	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	fmt.Printf("ATTEMPTING TO QUERY :: %s\n", to.GetAddress().Address)
	resp, err := c.Query(ctx, &pb.QueryRequest{Limit: 20, Sender: &n.self, Target: &find, Pingback: true})
	if err != nil {
		fmt.Printf("FAILED TO QUERY :: %s\n", to.GetAddress().Address)
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
	c, err := n.pool.Dial(ctx, &to)
	if err != nil {
		return false, NodeClientErr.Wrap(err)
	}

	_, err = c.Ping(ctx, &pb.PingRequest{})
	if err != nil {
		return false, err
	}

	return true, nil
}

// Disconnect closes all connections within the pool
func (n *Node) Disconnect() error {
	return n.pool.DisconnectAll()
}
