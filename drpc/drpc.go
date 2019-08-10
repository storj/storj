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
	Canceled      = errs.Class("canceled")
)

type Message interface {
	Reset()
	String() string
	ProtoMessage()
}

type Client interface {
	Transport() io.ReadWriteCloser

	Invoke(ctx context.Context, rpc string, in, out Message) error
	NewStream(ctx context.Context, rpc string) (Stream, error)
}

type Stream interface {
	Send(msg Message) error
	CloseSend() error

	Recv(msg Message) error
	CloseRecv() error
}
