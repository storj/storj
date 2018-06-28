// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transportclient

import (
	"context"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	proto "storj.io/storj/protos/overlay"

	"google.golang.org/grpc"
)

var (
	mon   = monkit.Package()
	Error = errs.Class("error")
)

// TransportClient defines the interface to an network client.
type TransportClient interface {
	DialUnauthenticated(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
}

// transportClient is the concrete implementation of the networkclient interface
type transportClient struct {
}
