// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"
	"log"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pool"
	"storj.io/storj/pkg/transport"
)

// Node is the storj definition for a node in the network
type Node struct {
	dht   dht.DHT
	self  pb.Node
	tc    transport.Client
	cache pool.Pool
}

// Lookup queries nodes looking for a particular node in the network
func (n *Node) Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error) {
	c, err := n.getConnection(to)
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
	_, err := n.getConnection(to)
	if err != nil {
		return false, NodeClientErr.Wrap(err)
	}

	return true, nil
}

func (n *Node) getConnection(to pb.Node) (pb.NodesClient, error) {
	v, err := n.cache.Get(ctx, to.GetId())
	if err != nil {
		return nil, NodeClientErr.Wrap(err)
	}

	var conn *grpc.ClientConn
	if c, ok := v.(*grpc.ClientConn); ok {
		conn = c
	} else {
		c, err := n.tc.DialNode(ctx, &to)
		if err != nil {
			return nil, NodeClientErr.Wrap(err)
		}

		if err := n.cache.Add(ctx, to.GetId(), c); err != nil {
			log.Printf("Error %s occurred adding %s to cache", err, to.GetId())
		}
		conn = c
	}

	return pb.NewNodesClient(conn), nil
}
