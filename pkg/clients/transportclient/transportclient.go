// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transportclient

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/connectivity"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

// TransportClient defines the interface to any client wanting to open a gRPC connection.
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
	/* TODO@ASK add security in after the security layer is finished. It is a filler but working code */
	if node == nil {
		return nil, errors.New("node param uninitialized")
	} else {
		/* TODO@ASK A dozen attempts... Recommendation: this value should be configurable */
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
				return nil, errors.New("node Address uninitialized")
			}
		} else {
			conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}
		}

		/* connection retry attempt */
		for (conn.GetState() != connectivity.State(connectivity.Ready)) && ((maxAttempts <= 12) && (maxAttempts > 0)) {
			time.Sleep(15 * time.Millisecond)
			maxAttempts--
		}
		if maxAttempts <= 0 {
			return nil, errors.New("Connection failed to open using grpc")
		}
		return conn, err
	}
}

// Dial using unauthenticated mode
func (o *transportClient) DialUnauthenticated(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	if node == nil {
		return nil, errors.New("node param uninitialized")
	} else {
		/* A dozen attempts... Recommendation: this value should be configurable */
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
				return nil, errors.New("node Address uninitialized")
			}
		} else {
			conn, err = grpc.Dial(node.Address.Address, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}
		}

		/* connection retry attempt */
		for (conn.GetState() != connectivity.State(connectivity.Ready)) && ((maxAttempts <= 12) && (maxAttempts > 0)) {
			time.Sleep(15 * time.Millisecond)
			maxAttempts--
		}
		if maxAttempts <= 0 {
			return nil, errors.New("Connection failed to open using grpc")
		}
		return conn, err
	}
}
