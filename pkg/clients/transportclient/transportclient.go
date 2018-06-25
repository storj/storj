// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transportclient

import (
	"context"
	"time"

	"google.golang.org/grpc/connectivity"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

// TransportClient defines the interface to an network client.
type TransportClient interface {
	DialUnauthenticated(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)
}

// transportClient is the concrete implementation of the networkclient interface
type transportClient struct {
	overlayClient *overlay.Overlay
}

//NewTransportClient returns TransportClient interface
func NewTransportClient(ctx context.Context, addr string) (TransportClient, error) {
	c, err := overlay.NewOverlayClient(addr)
	if err != nil {
		return nil, err
	}
	return &transportClient{
		overlayClient: c,
	}, err
}

// Dial using the authenticated mode
func (o *transportClient) DialNode(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	if node == nil {
		zap.S().Errorf("node param uninitialized : %v\n", err)
		return nil, err
	} else {
		maxAttempts := 12
		if node.Address == nil {
			/* check to see nodeID is present to look up for the corresponding address */
			if node.GetId() != "" {
				lookupNode, err := o.overlayClient.Lookup(ctx, overlay.NodeID(node.GetId()))
				if err != nil {
					return nil, err
				}
				/* err is nil, that means lookup passed complete info */
				conn, err = grpc.Dial(lookupNode.Address.Address, grpc.WithInsecure())
				if err != nil {
					return nil, err
				}
				node.Address.Address = lookupNode.Address.Address
			} else {
				zap.S().Errorf("node Address uninitialized : %v\n", err)
				return nil, err
			}
		} else {
			conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}

			for conn.GetState() != connectivity.State(connectivity.Ready) && maxAttempts < 12 {
				time.Sleep(15 * time.Millisecond)
				maxAttempts--
			}
		}
		if maxAttempts <= 0 {
			zap.S().Errorf("Connection failed with grpc : %v\n", err)
			return nil, err
		}
		return conn, err
	}
}

// Dial using unauthenticated mode
func (o *transportClient) DialUnauthenticated(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	if node == nil {
		zap.S().Errorf("node param uninitialized : %v\n", err)
		return nil, err
	} else {
		maxAttempts := 12
		if node.Address == nil {
			/* check to see nodeID is present to look up for the corresponding address */
			if node.GetId() != "" {
				lookupNode, err := o.overlayClient.Lookup(ctx, overlay.NodeID(node.GetId()))
				if err != nil {
					return nil, err
				}
				/* err is nil, that means lookup passed complete info */
				conn, err = grpc.Dial(lookupNode.Address.Address, grpc.WithInsecure())
				if err != nil {
					return nil, err
				}
				node.Address.Address = lookupNode.Address.Address
			} else {
				zap.S().Errorf("node Address uninitialized : %v\n", err)
				return nil, err
			}
		} else {
			conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}

			for conn.GetState() != connectivity.State(connectivity.Ready) && maxAttempts < 12 {
				time.Sleep(15 * time.Millisecond)
				maxAttempts--
			}
		}
		if maxAttempts <= 0 {
			zap.S().Errorf("Connection failed with grpc : %v\n", err)
			return nil, err
		}
		return conn, err
	}
}
