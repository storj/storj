// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/storj/drpc/drpctest"
	"storj.io/storj/drpc/drpcwire"
)

func TestVarint(t *testing.T) {
	t.Run("Round Trip", func(t *testing.T) {
		for i := 0; i < 64; i++ {
			// val has i+1 lower bits set
			val := (uint64(1) << uint(i+1)) - 1

			// the encoding should be related to the number of bits set
			buf := drpcwire.AppendVarint(nil, val)
			require.Equal(t, (i/7)+1, len(buf))

			// it should decode to the same value
			gotBuf, gotVal, ok, err := drpcwire.ReadVarint(buf)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, 0, len(gotBuf))
			require.Equal(t, val, gotVal)
		}
	})

	t.Run("Round Trip Fuzz", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			val := drpctest.RandUint64()
			buf := drpcwire.AppendVarint(nil, val)
			gotBuf, gotVal, ok, err := drpcwire.ReadVarint(buf)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, 0, len(gotBuf))
			require.Equal(t, val, gotVal)
		}
	})
}
