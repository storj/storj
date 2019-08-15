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
	sig       *drpcutil.Signal
	lbuf      *drpcutil.Buffer
	queue     chan *drpcwire.Packet
	sendMu    sync.Mutex
	sendSig   *drpcutil.Signal
	recvSig   *drpcutil.Signal
}

func New(streamID uint64, lbuf *drpcutil.Buffer) *Stream {
	return &Stream{
		streamID: streamID,
		sig:      drpcutil.NewSignal(),
		lbuf:     lbuf,
		queue:    make(chan *drpcwire.Packet, 100),
		sendSig:  drpcutil.NewSignal(),
		recvSig:  drpcutil.NewSignal(),
	}
}

var _ drpc.Stream = (*Stream)(nil)

//
// exported accessors
//

func (s *Stream) StreamID() uint64             { return s.streamID }
func (s *Stream) Sig() *drpcutil.Signal        { return s.sig }
func (s *Stream) SendSig() *drpcutil.Signal    { return s.sendSig }
func (s *Stream) RecvSig() *drpcutil.Signal    { return s.recvSig }
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

//
// raw send/recv/close primitives
//

func (s *Stream) RawSend(kind drpcwire.PayloadKind, data []byte) error {
	err := drpcwire.Split(kind, s.nextPid(), data, func(pkt drpcwire.Packet) error {
		s.sendMu.Lock()
		defer s.sendMu.Unlock()

		select {
		case <-s.sig.Signal():
			return s.sig.Err()
		case <-s.sendSig.Signal():
			return s.sendSig.Err()
		default:
		}

		return s.lbuf.Write(pkt)
	})
	if err != nil {
		s.sig.SignalWithError(err)
	}
	return err
}

func (s *Stream) RawFlush() error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	select {
	case <-s.sig.Signal():
		return s.sig.Err()
	case <-s.sendSig.Signal():
		return s.sendSig.Err()
	default:
	}

	err := s.lbuf.Flush()
	if err != nil {
		s.sig.SignalWithError(err)
	}
	return err
}

func (s *Stream) RawRecv() (*drpcwire.Packet, error) {
	select {
	case <-s.sig.Signal():
		return nil, s.sig.Err()
	default:
		select {
		case <-s.sig.Signal():
			return nil, s.sig.Err()
		case p, ok := <-s.queue:
			if !ok {
				return nil, io.EOF
			}
			return p, nil
		}
	}
}

func (s *Stream) RawCloseRecv() {
	if s.recvSig.SignalWithError(nil) {
		close(s.queue)
	}
}

func (s *Stream) RawCancel() error {
	// TODO: make this a thing
	return errs.New("TODO")
}

//
// drpc.Stream implementation
//

func (s *Stream) MsgSend(msg drpc.Message) error {
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

func (s *Stream) MsgRecv(msg drpc.Message) error {
	p, err := s.RawRecv()
	if err != nil {
		return err
	}
	return proto.Unmarshal(p.Data, msg)
}

func (s *Stream) CloseSend() error {
	// we don't use the Raw* functions so that we can hold the mutex the whole time
	// ensuring that we are the unique senders of Close, and that anyone else will
	// see the send signal closed.

	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	select {
	case <-s.sig.Signal():
		return s.sig.Err()
	case <-s.sendSig.Signal():
		return nil
	default:
	}

	defer s.sendSig.SignalWithError(drpc.Error.New("attempted to issue a send after CloseSend"))
	if err := drpcwire.Split(drpcwire.PayloadKind_Close, s.nextPid(), nil, s.lbuf.Write); err != nil {
		s.sig.SignalWithError(err)
		return err
	}
	if err := s.lbuf.Flush(); err != nil {
		s.sig.SignalWithError(err)
		return err
	}
	return nil
}

func (s *Stream) Close() error {
	defer s.sig.SignalWithError(drpc.StreamClosed.New(""))

	select {
	case <-s.sig.Signal():
		if err := s.sig.Err(); !drpc.StreamClosed.Has(err) {
			return err
		}
		return nil
	default:
		select {
		case <-s.sig.Signal():
			if err := s.sig.Err(); !drpc.StreamClosed.Has(err) {
				return err
			}
			return nil
		case <-s.sendSig.Signal():
			return nil
		default:
			return s.CloseSend()
		}
	}
}
