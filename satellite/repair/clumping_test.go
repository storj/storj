// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repair_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair"
)

func TestFindClumpedPieces(t *testing.T) {
	pieces := make(metabase.Pieces, 10)
	for n := range pieces {
		pieces[n] = metabase.Piece{
			Number:      0,
			StorageNode: testrand.NodeID(),
		}
	}

	t.Run("all-separate-nets", func(t *testing.T) {
		lastNets := make([]string, len(pieces))
		for n := range lastNets {
			lastNets[n] = fmt.Sprintf("172.16.%d.0", n)
		}
		clumped := repair.FindClumpedPieces(pieces, lastNets)
		require.Len(t, clumped, 0)
	})

	t.Run("one-clumped", func(t *testing.T) {
		lastNets := make([]string, len(pieces))
		for n := range lastNets {
			lastNets[n] = fmt.Sprintf("172.16.%d.0", n)
		}
		lastNets[len(lastNets)-1] = lastNets[0]
		clumped := repair.FindClumpedPieces(pieces, lastNets)
		require.Equal(t, metabase.Pieces{pieces[len(pieces)-1]}, clumped)
	})

	t.Run("all-clumped", func(t *testing.T) {
		lastNets := make([]string, len(pieces))
		for n := range lastNets {
			lastNets[n] = "172.16.41.0"
		}
		clumped := repair.FindClumpedPieces(pieces, lastNets)
		require.Equal(t, pieces[1:], clumped)
	})

	t.Run("two-clumps", func(t *testing.T) {
		lastNets := make([]string, len(pieces))
		for n := range lastNets {
			lastNets[n] = fmt.Sprintf("172.16.%d.0", n)
			lastNets[2] = lastNets[0]
			lastNets[4] = lastNets[1]
		}
		clumped := repair.FindClumpedPieces(pieces, lastNets)
		require.Equal(t, metabase.Pieces{pieces[2], pieces[4]}, clumped)
	})
}
