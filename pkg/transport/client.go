// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"
	"errors"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	proto "storj.io/storj/protos/overlay"
)

var (
	mon   = monkit.Package()
	Error = errs.Class("error")
)

// TransportClient defines the interface to an network client.
type Client interface {
	DialUnauthenticated(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
}

// transportClient is the concrete implementation of the networkclient interface
type client struct {
}

// Dial using the authenticated mode
func (o *client) DialNode(ctx context.Context, node proto.Node) (conn *grpc.ClientConn, err error) {

	/* TODO@ASK security feature under development */
	return o.DialUnauthenticated(ctx, node)
}

// Dial using unauthenticated mode
func (o *client) DialUnauthenticated(ctx context.Context, node proto.Node) (conn *grpc.ClientConn, err error) {
	if node.Address.Address == "" {
		return nil, errors.New("No Address")
	}

	conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return conn, err
}
