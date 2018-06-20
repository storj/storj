// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transportclient

import (
	"context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

// NetworkClient defines the interface to an network client.
type NetworkClient interface {
	DialUnauthenticated(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)
}

// TransportClient is the concrete implementation of the networkclient interface
type TransportClient struct {
	Conn *grpc.ClientConn
	Cc   *overlay.Overlay
}

// Dial using the authenticated mode
func (o *TransportClient) DialNode(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	o.Conn = nil
	/* check to see if address is empty? */
	if node.Address.Address == "" {
		/* check to see nodeID is present to look up for the corresponding address */
		if node.Id != "" {
			lookupNode, err := o.Cc.Lookup(ctx, overlay.NodeID(node.Id))
			conn, err = grpc.Dial(lookupNode.Address.Address, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
		o.Conn = conn
		if err != nil {
			return nil, err
		}
	}

	return conn, err
}

// Dial using unauthenticated mode
func (o *TransportClient) DialUnauthenticated(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	o.Conn = nil
	/* check to see if address is empty? */
	if node.Address.Address == "" {
		/* check to see nodeID is present to look up for the corresponding address */
		if node.Id != "" {
			lookupNode, err := o.Cc.Lookup(ctx, overlay.NodeID(node.Id))
			conn, err = grpc.Dial(lookupNode.Address.Address, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	}

	o.Conn = conn
	return conn, err
}
