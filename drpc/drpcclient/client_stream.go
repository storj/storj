// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcclient

import (
	"sync"
	"sync/atomic"

	"github.com/gogo/protobuf/proto"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcwire"
)

type clientStream struct {
	messageID uint64
	streamID  uint64

	sigerr
	client *Client
	queue  chan *drpcwire.Packet
	sendmu sync.Mutex
	send   sigerr
	recv   sigerr
}

var _ drpc.Stream = (*clientStream)(nil)

//
// basic helpers
//

func (cs *clientStream) nextPid() drpcwire.PacketID {
	return drpcwire.PacketID{
		StreamID:  cs.streamID,
		MessageID: atomic.AddUint64(&cs.messageID, 1),
	}
}

func (cs *clientStream) try(cb func() error) error {
	select {
	case <-cs.sig:
		return cs.err
	case <-cs.client.sig:
		return cs.client.err
	default:
		return cb()
	}
}

func (cs *clientStream) sendPollClosed() error {
	select {
	case <-cs.sig:
		return cs.err
	case <-cs.client.sig:
		return cs.client.err
	case <-cs.send.sig:
		return cs.send.err
	default:
		return nil
	}
}

//
// raw send/recv/close primitives
//

func (cs *clientStream) rawSend(kind drpcwire.PayloadKind, data []byte) error {
	return drpcwire.Split(kind, cs.nextPid(), data, func(pkt drpcwire.Packet) error {
		cs.sendmu.Lock()
		defer cs.sendmu.Unlock()

		if err := cs.sendPollClosed(); err != nil {
			return err
		}
		return cs.client.buf.Write(pkt)
	})
}

func (cs *clientStream) rawFlush() error {
	cs.sendmu.Lock()
	defer cs.sendmu.Unlock()

	if err := cs.sendPollClosed(); err != nil {
		return err
	}
	return cs.client.buf.Flush()
}

func (cs *clientStream) rawRecv() (*drpcwire.Packet, error) {
	select {
	case <-cs.sig:
		return nil, cs.err
	case <-cs.client.sig:
		return nil, cs.client.err
	case <-cs.recv.sig:
		return nil, cs.recv.err
	case p := <-cs.queue:
		return p, nil
	}
}

func (cs *clientStream) rawClose() error {
	cs.sendmu.Lock()
	defer cs.sendmu.Unlock()

	if err := cs.try(func() error {
		return drpcwire.Split(drpcwire.PayloadKind_Error, cs.nextPid(), nil,
			func(pkt drpcwire.Packet) error { return cs.client.buf.Write(pkt) })
	}); err != nil {
		cs.signalWithError(err)
		return err
	}
	if err := cs.try(func() error { return cs.client.buf.Flush() }); err != nil {
		cs.signalWithError(err)
		return err
	}
	return nil
}

//
// drpc.Stream implementation
//

func (cs *clientStream) Send(msg drpc.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	if err := cs.rawSend(drpcwire.PayloadKind_Message, data); err != nil {
		return err
	}
	if err := cs.rawFlush(); err != nil {
		return err
	}
	return nil
}

func (cs *clientStream) CloseSend() error {
	cs.signalWithError(drpc.Error.New("send after CloseSend"))
	select {
	case <-cs.recv.sig:
		return cs.rawClose()
	default:
		return nil
	}
}

func (cs *clientStream) Recv(msg drpc.Message) error {
	p, err := cs.rawRecv()
	if err != nil {
		return err
	}
	return proto.Unmarshal(p.Data, msg)
}

func (cs *clientStream) CloseRecv() error {
	cs.signalWithError(drpc.Error.New("recv after CloseRecv"))
	select {
	case <-cs.send.sig:
		return cs.rawClose()
	default:
		return nil
	}
}
