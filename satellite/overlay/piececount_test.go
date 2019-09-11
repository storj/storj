// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestOverlayDB_AllPieceCounts(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		overlaydb := db.OverlayCache()

		type TestNode struct {
			ID         storj.NodeID
			PieceCount int // TODO: fix to int64
		}

		nodes := make([]TestNode, 100)
		for i := range nodes {
			nodes[i].ID = testrand.NodeID()
			nodes[i].PieceCount = int(math.Pow10(i + 1))
		}

		for _, node := range nodes {
			require.NoError(t, overlaydb.UpdateAddress(ctx, &pb.Node{
				Id: node.ID,
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   "0.0.0.0",
				},
				LastIp: "0.0.0.0",
			}, overlay.NodeSelectionConfig{}))
		}

		// check that they are initialized to zero
		initialCounts, err := overlaydb.AllPieceCounts(ctx)
		require.NoError(t, err)
		require.Empty(t, initialCounts)
		// TODO: make it actually return everything
		// for _, node := range nodes {
		// 	count, ok := initialCounts[node.ID]
		// 	require.True(t, ok)
		// 	require.Equal(t, count, 0)
		// }

		// update counts
		counts := make(map[storj.NodeID]int)
		for _, node := range nodes {
			counts[node.ID] = node.PieceCount
		}
		err = overlaydb.UpdatePieceCounts(ctx, counts)
		require.NoError(t, err)

		// fetch new counts
		updatedCounts, err := overlaydb.AllPieceCounts(ctx)
		require.NoError(t, err)

		// verify values
		for _, node := range nodes {
			count, ok := updatedCounts[node.ID]
			require.True(t, ok)
			require.Equal(t, count, node.PieceCount)
		}
	})
}
