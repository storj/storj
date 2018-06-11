// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netclient

import (
	"context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/dtypes"
)

type NetClient interface {
	DialUnauthenticated(ctx context.Context, address dtypes.Address) (
		*grpc.ClientConn, error)
	DialNode(ctx context.Context, nodeID dtypes.Node) (*grpc.ClientConn, error)
}
