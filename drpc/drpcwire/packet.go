// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"fmt"

	"storj.io/storj/drpc"
)

//go:generate stringer -type=PayloadKind -trimprefix=PayloadKind_ -output=packet_string.go

// PayloadKind is an enumeration of all the different kinds of payloads.
type PayloadKind uint8

const (
	PayloadKind_Reserved PayloadKind = iota

	PayloadKind_Invoke    // body is rpc name
	PayloadKind_Message   // body is message data
	PayloadKind_Error     // body is error data
	PayloadKind_CloseSend // body must be empty
	PayloadKind_Cancel    // body must be empty

	PayloadKind_Largest
)

// MaxPacketSize is the maximum size of any packet on the wire.
const MaxPacketSize = 2 + 10 + 10 + 1023

//
// packet id
//

// PacketID contains two identifiers that uniquely identify a sequence of packets
// to form a message for some message in a stream.
type PacketID struct {
	StreamID  uint64
	MessageID uint64
}

// ParsePacketID parses a packet id out of buf. If there's not enough data for a full
// parse, ok will be false. If the parse fails then an error will be set. If the
// parse is successful, rem contains the remaining unused bytes.
func ParsePacketID(buf []byte) (rem []byte, pid PacketID, ok bool, err error) {
	if len(buf) < 2 {
		goto bad
	}

	rem, pid.StreamID, ok, err = ReadVarint(buf)
	if !ok || err != nil {
		goto bad
	}
	if pid.StreamID == 0 {
		err = drpc.ProtocolError.New("zero stream id")
		goto bad
	}
	rem, pid.MessageID, ok, err = ReadVarint(rem)
	if !ok || err != nil {
		goto bad
	}
	if pid.MessageID == 0 {
		err = drpc.ProtocolError.New("zero message id")
		goto bad
	}

	return rem, pid, true, nil
bad:
	return buf, pid, false, err
}

// AppendPacketID appends a byte form of the packet id to buf.
func AppendPacketID(buf []byte, pid PacketID) []byte {
	return AppendVarint(AppendVarint(buf, pid.StreamID), pid.MessageID)
}

//
// frame info
//

// FrameInfo contains information about a frame containing a possibly partial packet.
type FrameInfo struct {
	Length       uint16
	Continuation bool
	Starting     bool
	PayloadKind  PayloadKind
}

// ParseFrameInfo parses frame info out of buf. If there's not enough data for a full
// parse, ok will be false. If the parse fails then an error will be set. If the
// parse is successful, rem contains the remaining unused bytes.
func ParseFrameInfo(buf []byte) (rem []byte, fi FrameInfo, ok bool, err error) {
	var val uint16
	if len(buf) < 2 {
		goto bad
	}

	val = uint16(buf[0])<<8 | uint16(buf[1])
	fi.Length = val >> 6
	fi.Starting = val&(1<<5) > 0
	fi.Continuation = val&(1<<4) > 0
	fi.PayloadKind = PayloadKind(val & 15)

	return buf[2:], fi, true, nil
bad:
	return buf, fi, false, err
}

// AppendFrameInfo appends a byte form of the frame info to buf. It must not have
// a length larger than 1024 and must have a valid payload kind.
func AppendFrameInfo(buf []byte, fi FrameInfo) []byte {
	val := fi.Length << 6
	if fi.Starting {
		val |= 1 << 5
	}
	if fi.Continuation {
		val |= 1 << 4
	}
	val |= uint16(fi.PayloadKind)
	return append(buf, byte(val>>8), byte(val))
}

//
// header
//

// Header contains the header information common to every packet.
type Header struct {
	FrameInfo
	PacketID
}

// ParseHeader parses a packet header out of buf. If there's not enough data for a full
// parse, ok will be false. If the parse fails then an error will be set. If the
// parse is successful, rem contains the remaining unused bytes.
func ParseHeader(buf []byte) (rem []byte, hdr Header, ok bool, err error) {
	if len(buf) < 4 {
		goto bad
	}

	rem, hdr.FrameInfo, ok, err = ParseFrameInfo(buf)
	if !ok || err != nil {
		goto bad
	}
	rem, hdr.PacketID, ok, err = ParsePacketID(rem)
	if !ok || err != nil {
		goto bad
	}

	return rem, hdr, true, nil
bad:
	return buf, hdr, false, err
}

// AppendHeader appends a byte from of the header to buf. The frame info and packet
// id in the header must be valid.
func AppendHeader(buf []byte, hdr Header) []byte {
	return AppendPacketID(AppendFrameInfo(buf, hdr.FrameInfo), hdr.PacketID)
}

//
// packet
//

// Packet represents a possibly incomplete packet. External consumers of this library
// should only ever deal with complete packets.
type Packet struct {
	Header
	Data []byte
}

// ParsePacket parses a packet out of buf. If there's not enough data for a full
// parse, ok will be false. If the parse fails then an error will be set. If the
// parse is successful, rem contains the remaining unused bytes.
func ParsePacket(buf []byte) (rem []byte, pkt Packet, ok bool, err error) {
	var dataLen int
	if len(buf) < 4 {
		goto bad
	}

	rem, pkt.Header, ok, err = ParseHeader(buf)
	if !ok || err != nil {
		goto bad
	}
	dataLen = int(pkt.Length)
	if dataLen < 0 || len(rem) < dataLen {
		// dataLen < 0 is statically impossible, but the compiler needs
		// it to elide the bounds checks on rem. additionally, this
		// branch is not an error: we just have an incomplete packet.
		goto bad
	}
	pkt.Data = rem[:dataLen]

	return rem[dataLen:], pkt, true, nil
bad:
	return buf, pkt, false, err
}

// AppendPacket appends a byte form of the packet to buf.
func AppendPacket(buf []byte, pkt Packet) []byte {
	return append(AppendHeader(buf, pkt.Header), pkt.Data...)
}

func (p *Packet) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("pid:<%d,%d> kind:%v cont:%-5v start:%-5v len:%-4d data:%x",
		p.StreamID, p.MessageID,
		p.PayloadKind,
		p.Continuation,
		p.Starting,
		p.Length,
		p.Data,
	)
}
