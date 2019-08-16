// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcclient

import (
	"context"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcmanager"
	"storj.io/storj/drpc/drpcutil"
	"storj.io/storj/drpc/drpcwire"
)

type Client struct {
	sig *drpcutil.Signal
	man *drpcmanager.Manager
}

var _ drpc.Client = (*Client)(nil)

func New(ctx context.Context, rw io.ReadWriter) *Client {
	sig := drpcutil.NewSignal()
	man := drpcmanager.New(rw, nil)
	go func() { sig.Set(man.Run(ctx)) }()

	return &Client{
		sig: sig,
		man: man,
	}
}

func (c *Client) Close() error {
	c.man.Sig().Set(drpc.Error.New("client closed"))
	<-c.sig.Signal()
	return nil
}

func (c *Client) Invoke(ctx context.Context, rpc string, in, out drpc.Message) (err error) {
	data, err := proto.Marshal(in)
	if err != nil {
		return err
	}

	stream, err := c.man.NewStream(ctx, 0)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, stream.Close()) }()

	if err := stream.RawSend(drpcwire.PayloadKind_Invoke, []byte(rpc)); err != nil {
		return err
	}
	if err := stream.RawSend(drpcwire.PayloadKind_Message, data); err != nil {
		return err
	}
	if err := stream.CloseSend(); err != nil {
		return err
	}
	return stream.MsgRecv(out)
}

func (c *Client) NewStream(ctx context.Context, rpc string) (_ drpc.Stream, err error) {
	stream, err := c.man.NewStream(ctx, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, stream.Close())
		}
	}()

	if err := stream.RawSend(drpcwire.PayloadKind_Invoke, []byte(rpc)); err != nil {
		return nil, err
	}
	if err := stream.RawFlush(); err != nil {
		return nil, err
	}
	return stream, nil
}
