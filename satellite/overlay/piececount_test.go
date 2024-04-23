// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestDB_PieceCounts(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		overlaydb := db.OverlayCache()

		expectedNodePieces := make(map[storj.NodeID]int64, 100)

		for i := 0; i < 100; i++ {
			expectedNodePieces[testrand.NodeID()] = int64(math.Pow10(i + 1))
		}

		var nodeToDisqualify storj.NodeID

		i := 0
		for nodeID := range expectedNodePieces {
			addr := fmt.Sprintf("127.0.%d.0:8080", i)
			lastNet := fmt.Sprintf("127.0.%d", i)
			d := overlay.NodeCheckInInfo{
				NodeID:     nodeID,
				Address:    &pb.NodeAddress{Address: addr},
				LastIPPort: addr,
				LastNet:    lastNet,
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
				IsUp:       true,
			}
			err := overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
			i++

			nodeToDisqualify = nodeID
		}

		// check that they are initialized to zero
		initialCounts, err := overlaydb.ActiveNodesPieceCounts(ctx)
		require.NoError(t, err)
		require.Equal(t, len(expectedNodePieces), len(initialCounts))
		for nodeID := range expectedNodePieces {
			pieceCount, found := initialCounts[nodeID]
			require.True(t, found)
			require.Zero(t, pieceCount)
		}

		err = overlaydb.UpdatePieceCounts(ctx, expectedNodePieces)
		require.NoError(t, err)

		// fetch new counts
		updatedCounts, err := overlaydb.ActiveNodesPieceCounts(ctx)
		require.NoError(t, err)

		// verify values
		for nodeID, pieceCount := range expectedNodePieces {
			count, ok := updatedCounts[nodeID]
			require.True(t, ok)
			require.Equal(t, pieceCount, count)
		}

		// disqualify one node so it won't be returned by ActiveNodesPieceCounts
		_, err = overlaydb.DisqualifyNode(ctx, nodeToDisqualify, time.Now(), overlay.DisqualificationReasonAuditFailure)
		require.NoError(t, err)

		pieceCounts, err := overlaydb.ActiveNodesPieceCounts(ctx)
		require.NoError(t, err)
		require.NotContains(t, pieceCounts, nodeToDisqualify)
	})
}

func BenchmarkDB_PieceCounts(b *testing.B) {
	satellitedbtest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db satellite.DB) {
		var NumberOfNodes = 10000
		if testing.Short() {
			NumberOfNodes = 1000
		}

		overlaydb := db.OverlayCache()

		counts := make(map[storj.NodeID]int64)
		for i := 0; i < NumberOfNodes; i++ {
			counts[testrand.NodeID()] = testrand.Int63n(100000)
		}

		var i int
		for nodeID := range counts {
			addr := fmt.Sprintf("127.0.%d.0:8080", i)
			lastNet := fmt.Sprintf("127.0.%d", i)
			i++
			d := overlay.NodeCheckInInfo{
				NodeID:     nodeID,
				Address:    &pb.NodeAddress{Address: addr},
				LastIPPort: addr,
				LastNet:    lastNet,
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
			}
			err := overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(b, err)
		}

		b.Run("Update", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := overlaydb.UpdatePieceCounts(ctx, counts)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("All", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := overlaydb.ActiveNodesPieceCounts(ctx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
