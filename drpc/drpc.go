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
	Canceled      = errs.Class("canceled")
)

type Message interface {
	Reset()
	String() string
	ProtoMessage()
}

type Client interface {
	Invoke(ctx context.Context, rpc string, in, out Message) error
	NewStream(ctx context.Context, rpc string) (Stream, error)
}

type Stream interface {
	Send(msg Message) error
	Recv(msg Message) error
	CloseSend() error
	CloseRecv() error
	Close() error
}

type Handler = func(srv interface{}, ctx context.Context, in1, in2 interface{}) (out interface{}, err error)

type Description interface {
	NumMethods() int
	Method(n int) (rpc string, handler Handler, method interface{}, ok bool)
}
