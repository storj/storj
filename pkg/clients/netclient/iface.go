// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netclient

import (
	"context"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"storj.io/storj/pkg/dtypes"
)

// NetClient defines the interface to an overlay client.
type NetClient interface {
	DialUnauthenticated(ctx context.Context, address dtypes.Address) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, nodeID dtypes.Node) (*grpc.ClientConn, error)
}

// Overlay is the overlay concrete implementation of the client interface
type storjClient struct {
	nodeID dtypes.Node
	conn   *grpc.ClientConn
}

// Dial using the authenticated mode
func (o *storjClient) DialNode(ctx context.Context, nodeID dtypes.Node) (*grpc.ClientConn, error) {
	/* TODO@ASK: call the DHT functions to open up a connection to the DHT (cache) servers */
	conn, err := grpc.Dial((nodeID.Address.Network + strconv.Itoa(nodeID.Address.Port)), grpc.WithInsecure())
	if err != nil {
		zap.S().Errorf("error dialing: %v\n", err)
		return nil, err
	}
	return conn, err
}

// Dial using unauthenticated mode
func (o *storjClient) DialUnauthenticated(ctx context.Context, address dtypes.Address) (*grpc.ClientConn, error) {
	return nil, nil
}
