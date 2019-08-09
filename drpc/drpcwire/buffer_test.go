// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuffer(t *testing.T) {
	run := func(size int) func(t *testing.T) {
		return func(t *testing.T) {
			var exp []byte
			var got bytes.Buffer

			buffer := NewBuffer(&got, size)
			for i := 0; i < 1000; i++ {
				pkt := RandIncompletePacket()
				exp = AppendPacket(exp, pkt)
				require.NoError(t, buffer.Write(pkt))
			}
			require.NoError(t, buffer.Flush())
			require.Equal(t, exp, got.Bytes())

			// just ensures that the calls did not grow any internal buffers
			require.Equal(t, cap(buffer.buf), size)
			require.Equal(t, cap(buffer.tmp), MaxPacketSize)
		}
	}

	t.Run("0", run(0))
	t.Run(fmt.Sprint(MaxPacketSize), run(MaxPacketSize))
	t.Run("1MB", run(1024*1024))
}
