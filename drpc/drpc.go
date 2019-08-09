// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpc

import (
	"context"

	"github.com/zeebo/errs"
)

var (
	Error         = errs.Class("drpc")
	InternalError = errs.Class("internal error")
	ProtocolError = errs.Class("protocol error")
)

type Client interface {
	Invoke(ctx context.Context, rpc string, in, out interface{}) error
	NewStream(ctx context.Context, rpc string) (Stream, error)
	Close() error
}

type Stream interface {
	Send(msg interface{}) error
	CloseSend() error

	Recv(msg interface{}) error
	CloseRecv() error
}
