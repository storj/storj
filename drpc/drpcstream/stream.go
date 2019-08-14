// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcstream

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcutil"
	"storj.io/storj/drpc/drpcwire"
)

type Stream struct {
	messageID uint64
	streamID  uint64

	sig    *drpcutil.Signal
	lbuf   *drpcutil.LockedBuffer
	queue  chan *drpcwire.Packet
	mu     sync.Mutex
	send   *drpcutil.Signal
	recv   *drpcutil.Signal
	remote *drpcutil.Signal
}

func New(streamID uint64, lbuf *drpcutil.LockedBuffer) *Stream {
	return &Stream{
		messageID: 0,
		streamID:  streamID,

		sig:    drpcutil.NewSignal(),
		lbuf:   lbuf,
		queue:  make(chan *drpcwire.Packet, 100),
		send:   drpcutil.NewSignal(),
		recv:   drpcutil.NewSignal(),
		remote: drpcutil.NewSignal(),
	}
}

var _ drpc.Stream = (*Stream)(nil)

//
// exported accessors
//

func (s *Stream) StreamID() uint64             { return s.streamID }
func (s *Stream) Sig() *drpcutil.Signal        { return s.sig }
func (s *Stream) RecvSig() *drpcutil.Signal    { return s.recv }
func (s *Stream) SendSig() *drpcutil.Signal    { return s.send }
func (s *Stream) Remote() *drpcutil.Signal     { return s.remote }
func (s *Stream) Queue() chan *drpcwire.Packet { return s.queue }

//
// basic helpers
//

func (s *Stream) nextPid() drpcwire.PacketID {
	return drpcwire.PacketID{
		StreamID:  s.streamID,
		MessageID: atomic.AddUint64(&s.messageID, 1),
	}
}

func (s *Stream) try(cb func() error) error {
	select {
	case <-s.sig.Signal():
		return s.sig.Err()
	default:
		return cb()
	}
}

func (s *Stream) sendPollClosed() error {
	select {
	case <-s.sig.Signal():
		return s.sig.Err()
	case <-s.send.Signal():
		return s.send.Err()
	default:
		return nil
	}
}

func (s *Stream) recvPollClosed() error {
	select {
	case <-s.sig.Signal():
		return s.sig.Err()
	case <-s.recv.Signal():
		return s.send.Err()
	default:
		return nil
	}
}

//
// raw send/recv/close primitives
//

func (s *Stream) RawSend(kind drpcwire.PayloadKind, data []byte) error {
	return drpcwire.Split(kind, s.nextPid(), data, func(pkt drpcwire.Packet) error {
		s.mu.Lock()
		defer s.mu.Unlock()

		if err := s.sendPollClosed(); err != nil {
			return err
		}
		return s.lbuf.Write(pkt)
	})
}

func (s *Stream) RawFlush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.sendPollClosed(); err != nil {
		return err
	}
	return s.lbuf.Flush()
}

func (s *Stream) RawRecv() (*drpcwire.Packet, error) {
	if err := s.recvPollClosed(); err != nil {
		return nil, err
	}

	select {
	case <-s.sig.Signal():
		return nil, s.sig.Err()
	case <-s.recv.Signal():
		return nil, s.recv.Err()
	case p, ok := <-s.queue:
		if !ok {
			return nil, io.EOF
		}
		return p, nil
	}
}

func (s *Stream) RawCloseWithError(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err, ok := s.sig.State(); ok {
		return err
	}

	var data []byte
	if err != nil {
		data = []byte(err.Error())
	}

	if err := s.try(func() error {
		return drpcwire.Split(drpcwire.PayloadKind_Error, s.nextPid(), data,
			func(pkt drpcwire.Packet) error { return s.lbuf.Write(pkt) })
	}); err != nil {
		s.sig.SignalWithError(err)
		return err
	}

	if err := s.try(func() error { return s.lbuf.Flush() }); err != nil {
		s.sig.SignalWithError(err)
		return err
	}

	s.sig.SignalWithError(nil)
	return nil
}

//
// drpc.Stream implementation
//

func (s *Stream) Send(msg drpc.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	if err := s.RawSend(drpcwire.PayloadKind_Message, data); err != nil {
		return err
	}
	if err := s.RawFlush(); err != nil {
		return err
	}
	return nil
}

func (s *Stream) CloseSend() error {
	s.send.SignalWithError(drpc.Error.New("send after CloseSend"))
	if s.recv.WasSignaled() {
		return s.RawCloseWithError(nil)
	}
	return nil
}

func (s *Stream) Recv(msg drpc.Message) error {
	p, err := s.RawRecv()
	if err != nil {
		return err
	}
	return proto.Unmarshal(p.Data, msg)
}

func (s *Stream) CloseRecv() error {
	s.recv.SignalWithError(drpc.Error.New("recv after CloseRecv"))
	if s.send.WasSignaled() {
		return s.RawCloseWithError(nil)
	}
	return nil
}

func (s *Stream) Close() error {
	return errs.Combine(s.CloseSend(), s.CloseRecv())
}
