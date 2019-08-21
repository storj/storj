// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcconn

import (
	"context"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcmanager"
	"storj.io/storj/drpc/drpcutil"
	"storj.io/storj/drpc/drpcwire"
)

type Conn struct {
	once sync.Once
	tr   drpc.Transport
	sig  *drpcutil.Signal
	man  *drpcmanager.Manager
}

var _ drpc.Conn = (*Conn)(nil)

func New(tr drpc.Transport) *Conn {
	c := &Conn{
		tr:  tr,
		sig: drpcutil.NewSignal(),
		man: drpcmanager.New(tr, nil),
	}
	go c.monitorManager()
	return c
}

func (c *Conn) monitorManager() {
	c.sig.Set(c.man.Run(context.Background()))
}

func (c *Conn) Transport() drpc.Transport {
	return c.tr
}

func (c *Conn) Close() (err error) {
	c.man.Sig().Set(drpc.Error.New("transport closed"))
	c.once.Do(func() { err = c.tr.Close() })
	<-c.sig.Signal()
	return err
}

func (c *Conn) Invoke(ctx context.Context, rpc string, in, out drpc.Message) (err error) {
	data, err := proto.Marshal(in)
	if err != nil {
		return err
	}

	stream, err := c.man.NewStream(drpc.WithTransport(ctx, c.tr), 0)
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

func (c *Conn) NewStream(ctx context.Context, rpc string) (_ drpc.Stream, err error) {
	stream, err := c.man.NewStream(drpc.WithTransport(ctx, c.tr), 0)
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
