// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	proto "storj.io/storj/protos/overlay"
)

// transportClient is the concrete implementation of the networkclient interface
type Transport struct {
}

// Dial using the authenticated mode
func (o *Transport) DialNode(ctx context.Context, node proto.Node) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	/* TODO@ASK security feature under development */
	return o.DialUnauthenticated(ctx, node)
}

// Dial using unauthenticated mode
func (o *Transport) DialUnauthenticated(ctx context.Context, node proto.Node) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	if node.Address == nil || node.Address.Address == "" {
		return nil, errors.New("No Address")
	}

	return grpc.Dial(node.Address.Address, grpc.WithInsecure())
}
