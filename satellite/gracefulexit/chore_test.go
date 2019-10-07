// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/uplink"
)

func TestChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 8,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[1]

		satellite.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  6,
			SuccessThreshold: 8,
			MaxThreshold:     8,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		err = uplinkPeer.UploadWithConfig(ctx, satellite, rs, "testbucket", "test/path2", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitStatus := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now().UTC(),
		}

		_, err = satellite.Overlay.DB.UpdateExitStatus(ctx, &exitStatus)
		require.NoError(t, err)

		nodeIDs, err := satellite.Overlay.DB.GetExitingNodesLoopIncomplete(ctx)
		require.NoError(t, err)
		require.Len(t, nodeIDs, 1)

		satellite.GracefulExit.Chore.Loop.TriggerWait()

		incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 2)
		for _, incomplete := range incompleteTransfers {
			require.True(t, incomplete.DurabilityRatio > 0)
		}

		// test the other nodes don't have anything to transfer
		for _, sn := range planet.StorageNodes {
			if sn.ID() == exitingNode.ID() {
				continue
			}
			incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, sn.ID(), 20, 0)
			require.NoError(t, err)
			require.Len(t, incompleteTransfers, 0)
		}

		nodeIDs, err = satellite.Overlay.DB.GetExitingNodesLoopIncomplete(ctx)
		require.NoError(t, err)
		require.Len(t, nodeIDs, 0)
	})
}

func BenchmarkChore(b *testing.B) {
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		gracefulexitdb := db.GracefulExit()
		ctx := context.Background()

		b.Run("BatchUpdateStats-100", func(b *testing.B) {
			batch(ctx, b, gracefulexitdb, 100)
		})
		b.Run("BatchUpdateStats-250", func(b *testing.B) {
			batch(ctx, b, gracefulexitdb, 250)
		})
		b.Run("BatchUpdateStats-500", func(b *testing.B) {
			batch(ctx, b, gracefulexitdb, 500)
		})
		b.Run("BatchUpdateStats-1000", func(b *testing.B) {
			batch(ctx, b, gracefulexitdb, 1000)
		})
		b.Run("BatchUpdateStats-5000", func(b *testing.B) {
			batch(ctx, b, gracefulexitdb, 5000)
		})
	})
}
func batch(ctx context.Context, b *testing.B, db gracefulexit.DB, size int) {
	for i := 0; i < b.N; i++ {
		var transferQueueItems []gracefulexit.TransferQueueItem
		for j := 0; j < size; j++ {
			item := gracefulexit.TransferQueueItem{
				NodeID:          testrand.NodeID(),
				Path:            testrand.Bytes(memory.B * 256),
				PieceNum:        0,
				DurabilityRatio: 1.0,
			}
			transferQueueItems = append(transferQueueItems, item)
		}
		err := db.Enqueue(ctx, transferQueueItems)
		require.NoError(b, err)
	}
}
