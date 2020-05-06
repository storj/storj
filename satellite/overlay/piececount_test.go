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

		type TestNode struct {
			ID         storj.NodeID
			PieceCount int // TODO: fix to int64
		}

		nodes := make([]TestNode, 100)
		for i := range nodes {
			nodes[i].ID = testrand.NodeID()
			nodes[i].PieceCount = int(math.Pow10(i + 1))
		}

		for i, node := range nodes {
			addr := fmt.Sprintf("127.0.%d.0:8080", i)
			lastNet := fmt.Sprintf("127.0.%d", i)
			d := overlay.NodeCheckInInfo{
				NodeID:     node.ID,
				Address:    &pb.NodeAddress{Address: addr, Transport: pb.NodeTransport_TCP_TLS_GRPC},
				LastIPPort: addr,
				LastNet:    lastNet,
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
			}
			err := overlaydb.UpdateCheckIn(ctx, d, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
		}

		// check that they are initialized to zero
		initialCounts, err := overlaydb.AllPieceCounts(ctx)
		require.NoError(t, err)
		require.Empty(t, initialCounts)
		// TODO: make AllPieceCounts return results for all nodes,
		// since it will keep the logic slightly clearer.

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

func BenchmarkDB_PieceCounts(b *testing.B) {
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		ctx := testcontext.New(b)
		defer ctx.Cleanup()

		overlaydb := db.OverlayCache()

		counts := make(map[storj.NodeID]int)
		for i := 0; i < 10000; i++ {
			counts[testrand.NodeID()] = testrand.Intn(100000)
		}

		var i int
		for nodeID := range counts {
			addr := fmt.Sprintf("127.0.%d.0:8080", i)
			lastNet := fmt.Sprintf("127.0.%d", i)
			i++
			d := overlay.NodeCheckInInfo{
				NodeID:     nodeID,
				Address:    &pb.NodeAddress{Address: addr, Transport: pb.NodeTransport_TCP_TLS_GRPC},
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
				_, err := overlaydb.AllPieceCounts(ctx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
