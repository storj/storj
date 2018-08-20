// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"

	"google.golang.org/grpc"
	"storj.io/storj/pkg/pool"
	"storj.io/storj/pkg/transport"
	proto "storj.io/storj/protos/overlay"
)

// Node is the storj definition for a node in the network
type Node struct {
	self  proto.Node
	tc    transport.Client
	cache pool.Pool
}
//TODO: change lookup to findnear
// Lookup queries nodes looking for a particular node in the network
func (n *Node) Lookup(ctx context.Context, to proto.Node, find proto.Node) ([]*proto.Node, error) {
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

	c := proto.NewNodesClient(conn)
	resp, err := c.Query(ctx, &proto.QueryRequest{Sender: &n.self, Receiver: &find})
	if err != nil {
		return nil, err
	}

	return resp.Response, nil
}
