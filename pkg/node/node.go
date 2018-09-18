// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pool"
	"storj.io/storj/pkg/transport"
)

// Node is the storj definition for a node in the network
type Node struct {
	self  pb.Node
	tc    transport.Client
	cache pool.Pool
}

// Lookup queries nodes looking for a particular node in the network
func (n *Node) Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error) {
	v, err := n.cache.Get(ctx, to.GetId())
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
		conn = c
	}

	c := pb.NewNodesClient(conn)
	resp, err := c.Query(ctx, &pb.QueryRequest{Sender: &n.self, Target: &find})
	if err != nil {
		return nil, err
	}

	return resp.Response, nil
}
