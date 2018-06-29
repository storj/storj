// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	proto "storj.io/storj/protos/overlay"
)

var (
	mon = monkit.Package()
	//Error is the errs class of standard Transport Client errors
	Error = errs.Class("transport error")
)

// Client defines the interface to an transport client.
type Client interface {
	DialUnauthenticated(ctx context.Context, addr proto.NodeAddress) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
}
