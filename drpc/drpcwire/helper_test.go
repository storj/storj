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
	return PacketID{
		StreamID:  RandUint64() | 1,
		MessageID: RandUint64() | 1,
	}
}

func RandPayloadKind() PayloadKind {
	return PayloadKind(testrand.Intn(int(payloadKind_largest)-1) + 1)
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
	return Header{
		FrameInfo: RandFrameInfo(),
		PacketID:  RandPacketID(),
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
	return Packet{
		Header: hdr,
		Data:   testrand.BytesInt(rand.Intn(100 * 1024)),
	}
}
