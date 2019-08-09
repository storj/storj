// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"bufio"
	"io"

	"storj.io/storj/drpc"
)

type Receiver struct {
	scanner *bufio.Scanner
	pending map[PacketID]payloadState
	streams map[uint64]struct{}
}

type payloadState struct {
	invoke []byte
	data   []byte
	err    []byte
}

func NewReceiver(r io.Reader) *Receiver {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 32*1024), MaxPacketSize)
	scanner.Split(bufio.SplitFunc(func(data []byte, atEOF bool) (int, []byte, error) {
		rem, _, ok, err := ParsePacket(data)
		switch advance := len(data) - len(rem); {
		case err != nil, !ok:
			return 0, nil, err
		case advance < 0, len(data) < advance:
			return 0, nil, drpc.InternalError.New("bad parse")
		default:
			return advance, data[:advance], nil
		}
	}))
	return &Receiver{
		scanner: scanner,
		pending: make(map[PacketID]payloadState),
		streams: make(map[uint64]struct{}),
	}
}

// ReadPacket reads a fully formed Packet from the underlying reader and returns it. It
// handles message cancellation
func (r *Receiver) ReadPacket() (p *Packet, err error) {
restart:
	if !r.scanner.Scan() {
		return nil, drpc.Error.Wrap(r.scanner.Err())
	}

	// the scanner should return us exactly one packet, so if there's remaining
	// bytes or if it didn't parse, then there's some internal error.
	rem, pkt, ok, err := ParsePacket(r.scanner.Bytes())
	if err != nil {
		return nil, drpc.InternalError.Wrap(err)
	} else if !ok {
		return nil, drpc.InternalError.New("invalid data returned from scanner")
	} else if len(rem) != 0 {
		return nil, drpc.InternalError.New("remaining bytes from parsing packet")
	} else if len(pkt.Data) != int(pkt.Header.Length) {
		return nil, drpc.InternalError.New("invalid length of data and header length")
	}

	// get the payload state for the packet and ensure that the starting bit on the
	// frame is consistent with the payload state's existence.
	state, packetExists := r.pending[pkt.Header.PacketID]
	if !packetExists && !pkt.Header.Starting {
		return nil, drpc.ProtocolError.New("unknown packet id with no starting bit")
	} else if packetExists && pkt.Header.Starting {
		return nil, drpc.ProtocolError.New("starting packet id that already exists")
	}

	// require that the stream already exists if we're not starting. we allow the
	// stream to already exist if starting is true so that messages can be started
	// within an already started stream.
	_, streamExists := r.streams[pkt.Header.PacketID.StreamID]
	if !streamExists && !pkt.Header.Starting {
		return nil, drpc.ProtocolError.New("unknown stream id with no starting bit")
	}

	// handle cancel calls: they must have no body, not be continued, and the behavior
	// is different depending on if a message id is present.
	if pkt.Header.PayloadKind == PayloadKind_Cancel {
		switch {
		case len(pkt.Data) > 0:
			return nil, drpc.ProtocolError.New("data sent with cancel request")
		case pkt.Header.Continuation:
			return nil, drpc.ProtocolError.New("continuation bit set on cancel request")
		case pkt.Header.MessageID != 0:
			delete(r.pending, pkt.Header.PacketID)
			goto restart
		default:
			delete(r.streams, pkt.Header.StreamID)
			for pid := range r.pending {
				if pid.StreamID == pkt.Header.StreamID {
					delete(r.pending, pid)
				}
			}
			return &pkt, nil
		}
	}

	// append the packet's data into the appropriate buffer. it's important to do
	// this even if the packet is complete because we don't want to pass memory to
	// a caller that can be shared with some other caller.
	var buffer *[]byte
	switch pkt.Header.PayloadKind {
	case PayloadKind_Invoke:
		buffer = &state.invoke
	case PayloadKind_MessageData:
		buffer = &state.data
	case PayloadKind_ErrorData:
		buffer = &state.err
	default:
		return nil, drpc.ProtocolError.New("unknown payload kind")
	}
	*buffer = append(*buffer, pkt.Data...)

	// if it's continued, store it. we can always create the stream because we checked
	// that the starting bit is consistent.
	if pkt.Header.Continuation {
		r.pending[pkt.Header.PacketID] = state
		r.streams[pkt.Header.PacketID.StreamID] = struct{}{}
		goto restart
	}

	// we have a complete packet. we no longer need any state about it, and we clear
	// out the frame info to only have the payload kind as it's the only valid field
	// for higher level consumers.
	delete(r.pending, pkt.Header.PacketID)
	pkt.Header.FrameInfo = FrameInfo{PayloadKind: pkt.Header.PayloadKind}
	pkt.Data = *buffer
	return &pkt, nil
}
