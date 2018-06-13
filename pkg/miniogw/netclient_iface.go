// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"

	"google.golang.org/grpc"
	"storj.io/storj/pkg/overlay"
)

// NetClient defines the interface to an overlay client.
type NetClient interface {
	DialUnauthenticated(ctx context.Context, address string) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, nodeID string) (*grpc.ClientConn, error)
}

// Overlay is the overlay concrete implementation of the client interface
type storjClient struct {
	dhtcAddr string
}

// Dial using the authenticated mode
func (o *storjClient) DialNode(ctx context.Context, nodeID string) (*grpc.ClientConn, error) {
	/* TODO@ASK: call the DHT functions to open up a connection to the DHT (cache) servers */
	return nil, nil
}

// Dial using unauthenticated mode
func (o *storjClient) DialUnauthenticated(ctx context.Context, address string) (*grpc.ClientConn, error) {
	c, err := overlay.NewOverlayClient(o.dhtcAddr)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return c, err
}
