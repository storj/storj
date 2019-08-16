// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpctest

import (
	"math"

	"storj.io/storj/drpc/drpcwire"
	"storj.io/storj/internal/testrand"
)

func RandUint64() uint64 {
	return uint64(testrand.Int63n(math.MaxInt64))<<1 + uint64(testrand.Intn(2))
}

func RandBool() bool {
	return testrand.Intn(2) == 0
}

func RandPacketID() drpcwire.PacketID {
	return drpcwire.PacketID{
		StreamID:  RandUint64() | 1,
		MessageID: RandUint64() | 1,
	}
}

func RandPayloadKind() drpcwire.PayloadKind {
	return drpcwire.PayloadKind(testrand.Intn(int(drpcwire.PayloadKind_Largest)-1) + 1)
}

// payloadMaxSize maps a payload kind to 1 more than the maximum number of bytes that can
// be sent with a packet with that kind.
var payloadMaxSize = map[drpcwire.PayloadKind]func() int{
	drpcwire.PayloadKind_Invoke:    func() int { return testrand.Intn(1023) + 1 },
	drpcwire.PayloadKind_Message:   func() int { return testrand.Intn(1023) + 1 },
	drpcwire.PayloadKind_Error:     func() int { return testrand.Intn(1023) + 1 },
	drpcwire.PayloadKind_CloseSend: func() int { return 0 },
	drpcwire.PayloadKind_Cancel:    func() int { return 0 },
}

var kindCanContinue = map[drpcwire.PayloadKind]bool{
	drpcwire.PayloadKind_Invoke:    true,
	drpcwire.PayloadKind_Message:   true,
	drpcwire.PayloadKind_Error:     true,
	drpcwire.PayloadKind_CloseSend: false,
	drpcwire.PayloadKind_Cancel:    false,
}

func RandFrameInfo() drpcwire.FrameInfo {
	kind := RandPayloadKind()
	return drpcwire.FrameInfo{
		Length:       uint16(payloadMaxSize[kind]()),
		Continuation: kindCanContinue[kind] && RandBool(),
		Starting:     !kindCanContinue[kind] || RandBool(),
		PayloadKind:  kind,
	}
}

func RandHeader() drpcwire.Header {
	return drpcwire.Header{
		FrameInfo: RandFrameInfo(),
		PacketID:  RandPacketID(),
	}
}

func RandIncompletePacket() drpcwire.Packet {
	hdr := RandHeader()
	return drpcwire.Packet{
		Header: hdr,
		Data:   testrand.BytesInt(int(hdr.Length)),
	}
}

func RandCompletePacket() drpcwire.Packet {
	hdr := RandHeader()
	hdr.FrameInfo = drpcwire.FrameInfo{PayloadKind: hdr.PayloadKind}
	return drpcwire.Packet{
		Header: hdr,
		Data:   testrand.BytesInt(100 * payloadMaxSize[hdr.PayloadKind]()),
	}
}
