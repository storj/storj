// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"math"
	"math/rand"

	"storj.io/storj/internal/testrand"
)

func RandUint64() uint64 {
	return uint64(testrand.Int63n(math.MaxInt64))<<1 + uint64(testrand.Intn(2))
}

func RandBool() bool {
	return testrand.Intn(2) == 0
}

func RandPacketID() PacketID {
	streamID := RandUint64()
	if streamID == 0 {
		streamID = 1
	}
	return PacketID{
		StreamID:  streamID,
		MessageID: RandUint64(),
	}
}

func RandPayloadKind() PayloadKind {
	return PayloadKind(testrand.Intn(4) + 1)
}

func RandFrameInfo() FrameInfo {
	return FrameInfo{
		Length:       uint16(testrand.Intn(1024)),
		Continuation: RandBool(),
		Starting:     RandBool(),
		PayloadKind:  RandPayloadKind(),
	}
}

func RandHeader() Header {
	fi := RandFrameInfo()
	pid := RandPacketID()
	if fi.PayloadKind == PayloadKind_Cancel {
		pid.MessageID = 0
		fi.Length = 0
	}
	return Header{
		FrameInfo: fi,
		PacketID:  pid,
	}
}

func RandIncompletePacket() Packet {
	hdr := RandHeader()
	return Packet{
		Header: hdr,
		Data:   testrand.BytesInt(int(hdr.Length)),
	}
}

func RandCompletePacket() Packet {
	hdr := RandHeader()
	hdr.FrameInfo = FrameInfo{PayloadKind: hdr.PayloadKind}
	n := 0
	if hdr.PayloadKind != PayloadKind_Cancel {
		n = rand.Intn(100 * 1024)
	}
	return Packet{
		Header: hdr,
		Data:   testrand.BytesInt(n),
	}
}
