// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcclient

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcstream"
	"storj.io/storj/drpc/drpcutil"
	"storj.io/storj/drpc/drpcwire"
)

type Client struct {
	streamID uint64

	sig     *drpcutil.Signal
	rw      io.ReadWriter
	mu      sync.Mutex
	streams map[uint64]*drpcstream.Stream
	recv    *drpcwire.Receiver
	lbuf    *drpcutil.Buffer
}

var _ drpc.Client = (*Client)(nil)

func New(rw io.ReadWriter) *Client {
	c := &Client{
		sig:     drpcutil.NewSignal(),
		rw:      rw,
		streams: make(map[uint64]*drpcstream.Stream),
		recv:    drpcwire.NewReceiver(rw),
		lbuf:    drpcutil.NewBuffer(rw, drpcwire.MaxPacketSize),
	}
	go c.fillStreamQueues()
	return c
}

func (c *Client) Close() error {
	c.sig.SignalWithError(drpc.Error.New("client closed"))
	return nil
}

func (c *Client) newStream(ctx context.Context) *drpcstream.Stream {
	stream := drpcstream.New(atomic.AddUint64(&c.streamID, 1), c.lbuf)
	c.mu.Lock()
	c.streams[stream.StreamID()] = stream
	c.mu.Unlock()
	go c.monitorStream(ctx, stream)
	return stream
}

func (c *Client) monitorStream(ctx context.Context, stream *drpcstream.Stream) {
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-c.sig.Signal():
		err = c.sig.Err()
	case <-stream.Sig().Signal():
		err = stream.Sig().Err()
	}

	c.mu.Lock()
	delete(c.streams, stream.StreamID())
	c.mu.Unlock()

	stream.Sig().SignalWithError(err)
}

func (c *Client) fillStreamQueues() {
	defer c.sig.SignalWithError(drpc.InternalError.New("fillStreamQueues exited with no error"))

	for {
		p, err := c.recv.ReadPacket()
		switch {
		case err != nil:
			c.sig.SignalWithError(err)
			return

		case p == nil:
			c.sig.SignalWithError(nil)
			return
		}

		c.mu.Lock()
		stream := c.streams[p.StreamID]
		c.mu.Unlock()

		switch {
		case stream == nil:

		case p.PayloadKind == drpcwire.PayloadKind_Invoke:
			stream.Sig().SignalWithError(drpc.ProtocolError.New("server sent invoke message"))

		case p.PayloadKind == drpcwire.PayloadKind_Error:
			stream.Sig().SignalWithError(errs.New("%s", p.Data))

		case p.PayloadKind == drpcwire.PayloadKind_Close:
			stream.RawCloseRecv()

			c.mu.Lock()
			delete(c.streams, stream.StreamID())
			c.mu.Unlock()

		default:
			// we do a double select to ensure that multiple loops of packets cannot
			// send into the queue multiple times when the client or stream is closed.
			select {
			case <-stream.Sig().Signal(): // stream dead: just drop the message
			case <-stream.RecvSig().Signal(): // remote already said stream is done: problem
				c.sig.SignalWithError(drpc.ProtocolError.New("server sent after CloseSend"))
				return
			case <-c.sig.Signal(): // client dead: we're done filling queues
				return
			default:
				select {
				case <-stream.Sig().Signal(): // stream dead: just drop the message
				case <-stream.RecvSig().Signal(): // remote already said stream is done: problem
					c.sig.SignalWithError(drpc.ProtocolError.New("server sent after CloseSend"))
					return
				case <-c.sig.Signal(): // client dead: we're done filling queues
					return
				case stream.Queue() <- p: // yay we passed the message
				default: // producer overan: kill stream
					stream.Sig().SignalWithError(drpc.Error.New("stream buffer full"))
				}
			}
		}
	}
}

func (c *Client) Invoke(ctx context.Context, rpc string, in, out drpc.Message) (err error) {
	data, err := proto.Marshal(in)
	if err != nil {
		return err
	}

	stream := c.newStream(ctx)
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
	stream := c.newStream(ctx)
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
