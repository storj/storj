// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcclient

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcwire"
)

type Client struct {
	streamID uint64

	sigerr
	rwc     io.ReadWriteCloser
	mu      sync.Mutex
	streams map[uint64]*clientStream
	recv    *drpcwire.Receiver
	buf     lockedBuffer
}

var _ drpc.Client = (*Client)(nil)

func New(rwc io.ReadWriteCloser) *Client {
	c := &Client{
		sigerr:  newSigerr(),
		rwc:     rwc,
		streams: make(map[uint64]*clientStream),
		recv:    drpcwire.NewReceiver(rwc),
		buf:     lockedBuffer{buf: drpcwire.NewBuffer(rwc, drpcwire.MaxPacketSize)},
	}
	go c.fillStreamQueues()
	return c
}

func (c *Client) Transport() io.ReadWriteCloser {
	return c.rwc
}

func (c *Client) Close() error {
	return c.closeWithError(drpc.Error.New("client closed"))
}

func (c *Client) closeWithError(err error) error {
	c.signalWithError(err)
	return c.rwc.Close()
}

func (c *Client) newStream(ctx context.Context) *clientStream {
	cs := &clientStream{
		streamID:  atomic.AddUint64(&c.streamID, 1),
		messageID: 0,

		sigerr: newSigerr(),
		client: c,
		queue:  make(chan *drpcwire.Packet, 100),
		send:   newSigerr(),
		recv:   newSigerr(),
	}

	c.mu.Lock()
	c.streams[cs.streamID] = cs
	c.mu.Unlock()
	go c.monitorStream(ctx, cs)

	return cs
}

func (c *Client) monitorStream(ctx context.Context, cs *clientStream) {
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-c.sig:
		err = c.err
	case <-cs.sig:
	}

	c.mu.Lock()
	delete(c.streams, cs.streamID)
	c.mu.Unlock()

	if err != nil {
		cs.signalWithError(err)
	}
}

func (c *Client) fillStreamQueues() {
	defer c.closeWithError(drpc.InternalError.New("fillStreamQueues exited with no error"))

	for {
		p, err := c.recv.ReadPacket()
		switch {
		case err != nil:
			c.closeWithError(err)
			return

		case p == nil:
			c.closeWithError(io.EOF)
			return
		}

		c.mu.Lock()
		cs := c.streams[p.StreamID]
		c.mu.Unlock()

		switch {
		case cs == nil:

		case p.PayloadKind == drpcwire.PayloadKind_Invoke:
			cs.signalWithError(drpc.ProtocolError.New("server sent invoke message"))

		case p.PayloadKind == drpcwire.PayloadKind_Error:
			err := io.EOF
			if len(p.Data) > 0 {
				err = errs.New("%s", p.Data)
			}
			cs.signalWithError(err)

		default:
			// we do a double select to ensure that multiple loops of packets cannot
			// send into the queue multiple times when the client or stream is closed.
			select {
			case <-cs.sig: // just drop the message
			case <-cs.recv.sig: // just drop the message
			case <-c.sig: // we're done filling queues
				return

			default:
				select {
				case <-cs.sig: // just drop the message
				case <-cs.recv.sig: // just drop the message
				case <-c.sig: // we're done filling queues
					return

				case cs.queue <- p: // yay we passed the message

				default: // producer overan: kill stream
					cs.signalWithError(drpc.Error.New("stream buffer full"))
				}
			}
		}
	}
}

func (c *Client) Invoke(ctx context.Context, rpc string, in, out drpc.Message) (err error) {
	stream := c.newStream(ctx)
	defer func() { err = errs.Combine(err, stream.CloseSend(), stream.CloseRecv()) }()

	if err := stream.rawSend(drpcwire.PayloadKind_Invoke, []byte(rpc)); err != nil {
		return err
	}
	if err := stream.Send(in); err != nil {
		return err
	}
	return stream.Recv(out)
}

func (c *Client) NewStream(ctx context.Context, rpc string) (_ drpc.Stream, err error) {
	stream := c.newStream(ctx)
	defer func() {
		if err != nil {
			err = errs.Combine(err, stream.CloseSend(), stream.CloseRecv())
		}
	}()

	if err := stream.rawSend(drpcwire.PayloadKind_Invoke, []byte(rpc)); err != nil {
		return nil, err
	}
	if err := stream.rawFlush(); err != nil {
		return nil, err
	}
	return stream, nil
}
