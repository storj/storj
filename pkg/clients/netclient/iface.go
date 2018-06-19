// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netclient

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"storj.io/storj/pkg/dtypes"
	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

// NetClient defines the interface to an overlay client.
type NetClient interface {
	DialUnauthenticated(ctx context.Context, address dtypes.Address) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, nodeID dtypes.Node) (*grpc.ClientConn, error)
}

// Overlay is the overlay concrete implementation of the client interface
type storjClient struct {
	nodeID overlay.NodeID // of type string
	conn   *grpc.ClientConn
	cc     *overlay.Overlay
}

func init() {
}

// Dial using the authenticated mode
func (o *storjClient) DialNode(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	if node.Address == nil {
		addr := "bootstrap.storj.io:7070"
		cc, err := overlay.NewOverlayClient(addr)

		if err != nil {
			return nil, err //Error.Wrap(err)
		}
		nodeAddr, err := cc.Lookup(ctx, overlay.NodeID(node.Id))
		conn, err = grpc.Dial(nodeAddr.Address.Address, grpc.WithInsecure())

		if err != nil {
			zap.S().Errorf("error dialing: %v\n", err)
			return nil, err
		}
	} else {
		conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())

		if err != nil {
			zap.S().Errorf("error dialing: %v\n", err)
			return nil, err
		}
	}

	return conn, err
}

// Dial using unauthenticated mode
func (o *storjClient) DialUnauthenticated(ctx context.Context, address dtypes.Address) (*grpc.ClientConn, error) {
	return nil, nil
}
