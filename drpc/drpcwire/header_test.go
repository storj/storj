// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendParse(t *testing.T) {
	requireGoodParse := func(t *testing.T, exp interface{}) func([]byte, interface{}, bool, error) {
		return func(rem []byte, got interface{}, ok bool, err error) {
			t.Helper()
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, 0, len(rem))
			require.Equal(t, exp, got)
		}
	}

	t.Run("PacketID_RoundTrip_Fuzz_NoMessageID", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			exp := RandPacketID()
			exp.MessageID = 0
			requireGoodParse(t, exp)(ParsePacketID(AppendPacketID(nil, exp)))
		}
	})

	t.Run("PacketID_RoundTrip_Fuzz_WithMessageID", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			exp := RandPacketID()
			requireGoodParse(t, exp)(ParsePacketID(AppendPacketID(nil, exp)))
		}
	})

	t.Run("FrameInfo_RoundTrip_Fuzz", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			exp := RandFrameInfo()
			requireGoodParse(t, exp)(ParseFrameInfo(AppendFrameInfo(nil, exp)))
		}
	})

	t.Run("Header_RoundTrip_Fuzz", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			exp := RandHeader()
			requireGoodParse(t, exp)(ParseHeader(AppendHeader(nil, exp)))
		}
	})

	t.Run("Packet_RoundTrip_Fuzz", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			exp := RandIncompletePacket()
			requireGoodParse(t, exp)(ParsePacket(AppendPacket(nil, exp)))
		}
	})
}
