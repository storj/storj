// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transportclient

import (
	"context"

	"google.golang.org/grpc"
	proto "storj.io/storj/protos/overlay"
)

// Dial using the authenticated mode
func (o *transportClient) DialNode(ctx context.Context, node proto.Node) (conn *grpc.ClientConn, err error) {

	/* TODO@ASK security feature under development */
	return o.DialUnauthenticated(ctx, node)
}

// Dial using unauthenticated mode
func (o *transportClient) DialUnauthenticated(ctx context.Context, node proto.Node) (conn *grpc.ClientConn, err error) {
	conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return conn, err
}
