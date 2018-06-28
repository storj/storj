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
	mon   = monkit.Package()
	Error = errs.Class("error")
)

// client defines the interface to an network client.
type Client interface {
	DialUnauthenticated(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node proto.Node) (*grpc.ClientConn, error)
}
