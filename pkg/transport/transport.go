// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"

	"google.golang.org/grpc"

	proto "storj.io/storj/protos/overlay"
)

// Transport interface structure
type Transport struct {
}

// NewClient returns a newly instantiated Transport Client
func NewClient() *Transport {
	return &Transport{}
}

// DialNode using the authenticated mode
func (o *Transport) DialNode(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	if node.Address == nil {
		return nil, Error.New("no address")
	}
	/* TODO@ASK security feature under development */
	return o.DialUnauthenticated(ctx, *node.Address)
}

// DialUnauthenticated using unauthenticated mode
func (o *Transport) DialUnauthenticated(ctx context.Context, addr proto.NodeAddress) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	if addr.Address == "" {
		return nil, Error.New("no address")
	}

	return grpc.Dial(addr.Address, grpc.WithInsecure())
}
