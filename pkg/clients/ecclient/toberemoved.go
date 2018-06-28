// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"

	"google.golang.org/grpc"

	proto "storj.io/storj/protos/overlay"
)

// TransportClient is temporarily defined here.
// TODO: remove it
type TransportClient interface {
	DialNode(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
}
