// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

// Node is the storj definition for a node in the network
type Node struct {
	dht   dht.DHT
	self  pb.Node
	tc    transport.Client
	cache *ConnectionPool
}

// Lookup queries nodes looking for a particular node in the network
func (n *Node) Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error) {
	v, err := n.cache.Get(to.GetId())
	if err != nil {
		return nil, err
	}

	var conn *grpc.ClientConn
	if c, ok := v.(*grpc.ClientConn); ok {
		conn = c
	} else {
		c, err := n.tc.DialNode(ctx, &to)
		if err != nil {
			return nil, err
		}

		if err := n.cache.Add(to.GetId(), c); err != nil {
			log.Printf("Error %s occurred adding %s to cache", err, to.GetId())
		}
		conn = c
	}

	c := pb.NewNodesClient(conn)
	resp, err := c.Query(ctx, &pb.QueryRequest{Limit: 20, Sender: &n.self, Target: &find, Pingback: true})
	if err != nil {
		return nil, err
	}

	rt, err := n.dht.GetRoutingTable(ctx)
	if err != nil {
		return nil, err
	}

	if err := rt.ConnectionSuccess(&to); err != nil {
		return nil, err

	}

	return resp.Response, nil
}

// Disconnect closes connections within the cache
func (n *Node) Disconnect() error {
	return n.cache.Disconnect()
}
