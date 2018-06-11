// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dhtcclient

import (
	"context"

	"google.golang.org/grpc"
	"storj.io/storj/pkg/dtypes"
)

func NewDHTCClient(*grpc.ClientConn) DHTCClient {
	panic("TODO")
}

type DHTCClient interface {
	Choose(ctx context.Context, amount int, space, bw int) ([]dtypes.Node, error)
	Lookup(ctx context.Context, nodeID dtypes.NodeID) (dtypes.Node, error)
}
