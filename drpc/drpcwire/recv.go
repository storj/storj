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
	}
}

// ReadPacket reads a fully formed Packet from the underlying reader and returns it. It
// handles message cancellation
func (r *Receiver) ReadPacket() (p *Packet, err error) {
	var (
		rem []byte
		pkt Packet
		ok  bool
	)

	for {
		if !r.scanner.Scan() {
			return nil, drpc.Error.Wrap(r.scanner.Err())
		}

		// the scanner should return us exactly one packet, so if there's remaining
		// bytes or if it didn't parse, then there's some internal error.
		rem, pkt, ok, err = ParsePacket(r.scanner.Bytes())
		if err != nil {
			return nil, drpc.InternalError.Wrap(err)
		} else if !ok {
			return nil, drpc.InternalError.New("invalid data returned from scanner")
		} else if len(rem) != 0 {
			return nil, drpc.InternalError.New("remaining bytes from parsing packet")
		} else if len(pkt.Data) != int(pkt.Length) {
			return nil, drpc.InternalError.New("invalid length of data and header length")
		}

		// handle stream closing/cancelation first. all we have to do is delete all the
		// pending state for anything that matches the stream id after validating it.
		if pkt.PayloadKind == PayloadKind_Cancel {
			if pkt.MessageID != 0 {
				return nil, drpc.ProtocolError.New("received cancel with non-zero message id")
			} else if len(pkt.Data) > 0 {
				return nil, drpc.ProtocolError.New("received cancel with data")
			} else if pkt.Continuation {
				return nil, drpc.ProtocolError.New("received cancel with continuation bit set")
			}

			for pid := range r.pending {
				if pid.StreamID == pkt.StreamID {
					delete(r.pending, pid)
				}
			}
			break
		}

		// get the payload state for the packet and ensure that the starting bit on the
		// frame is consistent with the payload state's existence.
		state, packetExists := r.pending[pkt.PacketID]
		if !packetExists && !pkt.Starting {
			return nil, drpc.ProtocolError.New("unknown packet id with no starting bit")
		} else if packetExists && pkt.Starting {
			return nil, drpc.ProtocolError.New("starting packet id that already exists")
		}

		// append the packet's data into the appropriate buffer. it's important to do
		// this even if the packet is complete because we don't want to pass memory to
		// a caller that can be shared with some other caller.
		var buffer *[]byte
		switch pkt.PayloadKind {
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

		// if we have a complete packet. we no longer need any state about it and the
		// packet is now complete, so we set the data to the completed buffer.
		if !pkt.Continuation {
			delete(r.pending, pkt.PacketID)
			pkt.Data = *buffer
			break
		}

		r.pending[pkt.PacketID] = state
	}

	// we clear out out the frame info to only have the payload kind as it's the only
	// valid field for higher level consumers.
	pkt.FrameInfo = FrameInfo{PayloadKind: pkt.PayloadKind}
	return &pkt, nil
}
