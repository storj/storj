// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcserver

import (
	"context"
	"io"
	"reflect"
	"sync"

	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcstream"
	"storj.io/storj/drpc/drpcutil"
	"storj.io/storj/drpc/drpcwire"
)

type session struct {
	rw      io.ReadWriter
	rpcs    map[string]rpcData
	mu      sync.Mutex
	streams map[uint64]*drpcstream.Stream
	recv    *drpcwire.Receiver
	lbuf    *drpcutil.LockedBuffer
}

func newSession(rw io.ReadWriter, rpcs map[string]rpcData) *session {
	return &session{
		rw:      rw,
		rpcs:    rpcs,
		streams: make(map[uint64]*drpcstream.Stream),
		recv:    drpcwire.NewReceiver(rw),
		lbuf:    drpcutil.NewLockedBuffer(drpcwire.NewBuffer(rw, drpcwire.MaxPacketSize)),
	}
}

func (s *session) newStream(streamID uint64) *drpcstream.Stream {
	stream := drpcstream.New(streamID, s.lbuf)
	s.mu.Lock()
	s.streams[stream.StreamID()] = stream
	s.mu.Unlock()
	go s.monitorStream(stream)
	return stream
}

func (s *session) monitorStream(stream *drpcstream.Stream) {
	<-stream.Sig().Signal()
	s.mu.Lock()
	delete(s.streams, stream.StreamID())
	s.mu.Unlock()
}

func (s *session) Run() error {
	for {
		p, err := s.recv.ReadPacket()
		// fmt.Println("rawsrv", p, err)
		switch {
		case err != nil:
			return err
		case p == nil:
			return nil
		}

		s.mu.Lock()
		stream := s.streams[p.StreamID]
		s.mu.Unlock()

		switch {
		case stream == nil && p.PayloadKind == drpcwire.PayloadKind_Invoke:
			data, ok := s.rpcs[string(p.Data)]
			if !ok {
				return drpc.ProtocolError.New("unknown rpc: %q", p.Data)
			}
			stream = s.newStream(p.StreamID)
			go s.runRPC(stream, data)

		case stream == nil:

		case p.PayloadKind == drpcwire.PayloadKind_Invoke:
			stream.Sig().SignalWithError(drpc.ProtocolError.New("invoke on an existing stream"))

		case p.PayloadKind == drpcwire.PayloadKind_Error:
			switch {
			case len(p.Data) > 0:
				stream.Sig().SignalWithError(errs.New("%s", p.Data))
			case stream.Remote().SignalWithError(nil):
				close(stream.Queue())
			default:
				stream.Sig().SignalWithError(drpc.ProtocolError.New("client sent after stream closed"))
			}

		default:
			// we do a double select to ensure that multiple loops of packets cannot
			// send into the queue multiple times when the client or stream is closed.
			select {
			case <-stream.Sig().Signal(): // stream dead: just drop the message
			case <-stream.RecvSig().Signal(): // stream closed recv: just drop the message
			case <-stream.Remote().Signal(): // remote already said stream is done: problem
				stream.Sig().SignalWithError(drpc.ProtocolError.New("server sent after stream closed"))
			default:
				select {
				case <-stream.Sig().Signal(): // stream dead: just drop the message
				case <-stream.RecvSig().Signal(): // stream closed recv: just drop the message
				case <-stream.Remote().Signal(): // remote already said stream is done: problem
					stream.Sig().SignalWithError(drpc.ProtocolError.New("server sent after stream closed"))
				case stream.Queue() <- p: // yay we passed the message
				default: // producer overan: kill stream
					stream.Sig().SignalWithError(drpc.Error.New("stream buffer full"))
				}
			}
		}
	}
}

func (s *session) runRPC(stream *drpcstream.Stream, data rpcData) {
	stream.RawCloseWithError(s.performRPC(stream, data))
}

func (s *session) performRPC(stream *drpcstream.Stream, data rpcData) (err error) {
	var in interface{} = stream
	if data.in1 != streamType {
		msg := reflect.New(data.in1.Elem()).Interface().(drpc.Message)
		if err := stream.Recv(msg); err != nil {
			return err
		}
		in = msg
	}

	out, err := data.handler(data.srv, context.Background(), in, stream)
	switch {
	case err != nil:
		return err
	case out != nil:
		return stream.Send(out.(drpc.Message))
	default:
		return nil
	}
}
