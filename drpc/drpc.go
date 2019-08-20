// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpc

import (
	"context"
	"io"

	"github.com/zeebo/errs"
)

var (
	Error         = errs.Class("drpc")
	InternalError = errs.Class("internal error")
	ProtocolError = errs.Class("protocol error")
	Closed        = errs.Class("closed")
)

type Transport interface {
	io.Reader
	io.Writer
	io.Closer
}

type Message interface {
	Reset()
	String() string
	ProtoMessage()
}

type Conn interface {
	Transport() Transport

	Invoke(ctx context.Context, rpc string, in, out Message) error
	NewStream(ctx context.Context, rpc string) (Stream, error)
}

type Stream interface {
	Context() context.Context

	MsgSend(msg Message) error
	MsgRecv(msg Message) error

	CloseSend() error
	Close() error
}

type Handler = func(srv interface{}, ctx context.Context, in1, in2 interface{}) (out Message, err error)

type Description interface {
	NumMethods() int
	Method(n int) (rpc string, handler Handler, method interface{}, ok bool)
}

type transportKey struct{}

func WithTransport(ctx context.Context, tr Transport) context.Context {
	return context.WithValue(ctx, transportKey{}, tr)
}

func TransportFromContext(ctx context.Context) (Transport, bool) {
	tr, ok := ctx.Value(transportKey{}).(Transport)
	return tr, ok
}
