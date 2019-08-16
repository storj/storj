// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmanager

import (
	"context"
	"io"
	"sync"

	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcstream"
	"storj.io/storj/drpc/drpcutil"
	"storj.io/storj/drpc/drpcwire"
)

type Handler interface {
	Handle(stream *drpcstream.Stream, rpc string) error
}

type Manager struct {
	mu       sync.Mutex
	streamID uint64
	handler  Handler
	sig      *drpcutil.Signal
	streams  map[uint64]*drpcstream.Stream
	recv     *drpcwire.Receiver
	buf      *drpcutil.Buffer
}

func New(rw io.ReadWriter, handler Handler) *Manager {
	return &Manager{
		handler: handler,
		sig:     drpcutil.NewSignal(),
		streams: make(map[uint64]*drpcstream.Stream),
		recv:    drpcwire.NewReceiver(rw),
		buf:     drpcutil.NewBuffer(rw, drpcwire.MaxPacketSize),
	}
}

func (m *Manager) Sig() *drpcutil.Signal { return m.sig }

func (m *Manager) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go m.monitorContext(ctx)
	go m.manageStreams(ctx)

	<-m.sig.Signal()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stream := range m.streams {
		<-stream.Sig().Signal()
	}
	return m.sig.Err()
}

func (m *Manager) monitorContext(ctx context.Context) {
	<-ctx.Done()
	m.sig.Set(ctx.Err())
}

func (m *Manager) NewStream(ctx context.Context, streamID uint64) (*drpcstream.Stream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, ok := m.sig.Get(); ok {
		return nil, err
	}

	if streamID == 0 {
		m.streamID++
		streamID = m.streamID
	}

	stream := drpcstream.New(ctx, streamID, m.buf)
	m.streams[streamID] = stream
	go m.monitorStream(stream)
	return stream, nil
}

func (m *Manager) monitorStream(stream *drpcstream.Stream) {
	select {
	case <-m.sig.Signal():
		stream.Sig().Set(m.sig.Err())
	case <-stream.Sig().Signal():
	}

	m.mu.Lock()
	delete(m.streams, stream.StreamID())
	m.mu.Unlock()
}

func (m *Manager) manageStreams(ctx context.Context) {
	defer m.sig.Set(drpc.InternalError.New("manageStreams exited with no signal"))

	for {
		p, err := m.recv.ReadPacket()
		switch {
		case err != nil:
			m.sig.Set(err)
			return
		case p == nil:
			m.sig.Set(io.EOF)
			return
		}

		m.mu.Lock()
		stream := m.streams[p.StreamID]
		m.mu.Unlock()

		switch {
		// manager error: we're done
		case m.sig.IsSet():
			return

		// invoke with no handler: protocol error
		case p.PayloadKind == drpcwire.PayloadKind_Invoke && m.handler == nil:
			m.sig.Set(drpc.ProtocolError.New("invalid invoke message to client"))
			return

		// invoke with a fresh stream: start up a handler
		case p.PayloadKind == drpcwire.PayloadKind_Invoke && stream == nil:
			stream, err := m.NewStream(ctx, p.StreamID)
			if err != nil {
				m.sig.Set(err)
				return
			}
			go m.handler.Handle(stream, string(p.Data))

		// no stream found: drop message
		case stream == nil:

		// invoke with an existing stream: double invoke
		case p.PayloadKind == drpcwire.PayloadKind_Invoke:
			m.sig.Set(drpc.ProtocolError.New("invoke on an existing stream"))
			return

		// close send: signal to the stream that no more sends will happen
		case p.PayloadKind == drpcwire.PayloadKind_CloseSend:
			if stream.RecvSig().Set(drpc.Closed.New("remote sent CloseSend")) {
				close(stream.Queue())
			}

		// cancel: signal to the stream that the remote side canceled
		case p.PayloadKind == drpcwire.PayloadKind_Cancel:
			stream.Cancel()

		// error: signal to the stream what the error is
		case p.PayloadKind == drpcwire.PayloadKind_Error:
			stream.Sig().Set(errs.New("%s", p.Data))

		// send after close send: protocol error
		case stream.RecvSig().IsSet():
			m.sig.Set(drpc.ProtocolError.New("remote sent message after CloseSend"))
			return

		// stream error: drop the message
		case stream.Sig().IsSet():

		default:
			select {
			// manager error: we're done
			case <-m.sig.Signal():
				return

			// send after close send: protocol error
			case <-stream.RecvSig().Signal():
				m.sig.Set(drpc.ProtocolError.New("remote sent message after CloseSend"))
				return

			// stream error: drop the message
			case <-stream.Sig().Signal():

			// attempt to place it into the queue
			case stream.Queue() <- p:

			// if we couldn't put it in the queue, the stream is full.
			default:
				stream.Sig().Set(drpc.ProtocolError.New("stream buffer full"))
			}
		}
	}
}
