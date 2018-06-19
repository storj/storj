// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transportclient

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

// TransportClient defines the interface to an overlay client.
type TransportClient interface {
	DialUnauthenticated(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)
}

// StorjClient is the overlay concrete implementation of the client interface
type StorjClient struct {
	NodeID overlay.NodeID // of type string
	Conn   *grpc.ClientConn
	Cc     *overlay.Overlay
}

func init() {
}

// Dial using the authenticated mode
func (o *StorjClient) DialNode(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	if node.Address == nil {
		addr := "bootstrap.storj.io:7070"
		o.Cc, err = overlay.NewOverlayClient(addr)
		if err != nil {
			o.Cc = nil
			zap.S().Errorf("Client conn successful: %v\n", err)
			return nil, err
		}

		if node.Id != "" {
			nodeAddr, err := o.Cc.Lookup(ctx, overlay.NodeID(node.Id))
			conn, err = grpc.Dial(nodeAddr.Address.Address, grpc.WithInsecure())
			if err != nil {
				zap.S().Errorf("error dialing: %v\n", err)
				return nil, err
			}
		} else {
			return nil, nil
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
func (o *StorjClient) DialUnauthenticated(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error) {
	return nil, nil
}
