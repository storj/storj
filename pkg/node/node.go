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
	dht  dht.DHT
	self pb.Node
	pool *ConnectionPool
}

// Lookup queries nodes looking for a particular node in the network
func (node *Node) Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error) {
	conn, err := node.pool.Dial(ctx, &to)
	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	resp, err := conn.Query(ctx, &pb.QueryRequest{
		Limit:    20,
		Sender:   &node.self,
		Target:   &find,
		Pingback: true,
	})

	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	rt, err := node.dht.GetRoutingTable(ctx)
	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	if err := rt.ConnectionSuccess(&to); err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	return resp.Response, nil
}

// Ping attempts to establish a connection with a node to verify it is alive
func (node *Node) Ping(ctx context.Context, to pb.Node) (bool, error) {
	conn, err := node.pool.Dial(ctx, &to)
	if err != nil {
		return false, NodeClientErr.Wrap(err)
	}

	_, err = conn.Ping(ctx, &pb.PingRequest{})
	if err != nil {
		return false, err
	}

	return true, nil
}

// Disconnect closes all connections within the pool
func (node *Node) Disconnect() error {
	return node.pool.DisconnectAll()
}
