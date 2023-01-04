// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProgress(t *testing.T) {
	// test basic graceful exit progress crud
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		geDB := db.GracefulExit()

		testData := []struct {
			nodeID      storj.NodeID
			bytes       int64
			transferred int64
			failed      int64
		}{
			{testrand.NodeID(), 10, 2, 1},
			{testrand.NodeID(), 1, 4, 0},
		}
		for _, data := range testData {
			err := geDB.IncrementProgress(ctx, data.nodeID, data.bytes, data.transferred, data.failed)
			require.NoError(t, err)

			progress, err := geDB.GetProgress(ctx, data.nodeID)
			require.NoError(t, err)
			require.Equal(t, data.bytes, progress.BytesTransferred)
			require.Equal(t, data.transferred, progress.PiecesTransferred)
			require.Equal(t, data.failed, progress.PiecesFailed)

			err = geDB.IncrementProgress(ctx, data.nodeID, 1, 1, 1)
			require.NoError(t, err)

			progress, err = geDB.GetProgress(ctx, data.nodeID)
			require.NoError(t, err)
			require.Equal(t, data.bytes+1, progress.BytesTransferred)
			require.Equal(t, data.transferred+1, progress.PiecesTransferred)
			require.Equal(t, data.failed+1, progress.PiecesFailed)
		}
	})
}

func TestSegmentTransferQueueItem(t *testing.T) {
	// test basic graceful exit transfer queue crud
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		geDB := db.GracefulExit()

		nodeID1 := testrand.NodeID()
		nodeID2 := testrand.NodeID()
		streamID1 := testrand.UUID()
		streamID2 := testrand.UUID()
		position1 := metabase.SegmentPosition{Part: 1, Index: 2}
		position2 := metabase.SegmentPosition{Part: 2, Index: 3}

		// root piece IDs for segments 1 and 2
		rootPieceID1 := testrand.PieceID()
		rootPieceID2 := testrand.PieceID()
		items := []gracefulexit.TransferQueueItem{
			{
				NodeID:          nodeID1,
				StreamID:        streamID1,
				Position:        position1,
				PieceNum:        1,
				RootPieceID:     rootPieceID1,
				DurabilityRatio: 0.9,
			},
			{
				NodeID:          nodeID1,
				StreamID:        streamID2,
				Position:        position2,
				PieceNum:        2,
				RootPieceID:     rootPieceID2,
				DurabilityRatio: 1.1,
			},
			{
				NodeID:          nodeID2,
				StreamID:        streamID1,
				Position:        position1,
				PieceNum:        2,
				RootPieceID:     rootPieceID1,
				DurabilityRatio: 0.9,
			},
			{
				NodeID:          nodeID2,
				StreamID:        streamID2,
				Position:        position2,
				PieceNum:        1,
				RootPieceID:     rootPieceID2,
				DurabilityRatio: 1.1,
			},
		}

		// test basic create, update, get delete
		{
			batchSize := 1000
			err := geDB.Enqueue(ctx, items, batchSize)
			require.NoError(t, err)

			for _, tqi := range items {
				item, err := geDB.GetTransferQueueItem(ctx, tqi.NodeID, tqi.StreamID, tqi.Position, tqi.PieceNum)
				require.NoError(t, err)
				require.Equal(t, tqi.RootPieceID, item.RootPieceID)
				require.Equal(t, tqi.DurabilityRatio, item.DurabilityRatio)

				now := time.Now()
				item.DurabilityRatio = 1.2
				item.RequestedAt = &now

				err = geDB.UpdateTransferQueueItem(ctx, *item)
				require.NoError(t, err)

				latestItem, err := geDB.GetTransferQueueItem(ctx, tqi.NodeID, tqi.StreamID, tqi.Position, tqi.PieceNum)
				require.NoError(t, err)

				require.Equal(t, item.RootPieceID, latestItem.RootPieceID)
				require.Equal(t, item.DurabilityRatio, latestItem.DurabilityRatio)
				require.WithinDuration(t, now, *latestItem.RequestedAt, time.Second)
			}

			queueItems, err := geDB.GetIncomplete(ctx, nodeID1, 10, 0)
			require.NoError(t, err)
			require.Len(t, queueItems, 2)
		}

		// mark the first item finished and test that only 1 item gets returned from the GetIncomplete
		{
			item, err := geDB.GetTransferQueueItem(ctx, nodeID1, streamID1, position1, 1)
			require.NoError(t, err)

			now := time.Now()
			item.FinishedAt = &now

			err = geDB.UpdateTransferQueueItem(ctx, *item)
			require.NoError(t, err)

			queueItems, err := geDB.GetIncomplete(ctx, nodeID1, 10, 0)
			require.NoError(t, err)
			require.Len(t, queueItems, 1)
			for _, queueItem := range queueItems {
				require.Equal(t, nodeID1, queueItem.NodeID)
				require.Equal(t, streamID2, queueItem.StreamID)
				require.Equal(t, position2, queueItem.Position)
			}
		}

		// test delete finished queue items. Only key1 should be removed
		{
			err := geDB.DeleteFinishedTransferQueueItems(ctx, nodeID1)
			require.NoError(t, err)

			// key1 should no longer exist for nodeID1
			_, err = geDB.GetTransferQueueItem(ctx, nodeID1, streamID1, position1, 1)
			require.Error(t, err)

			// key2 should still exist for nodeID1
			_, err = geDB.GetTransferQueueItem(ctx, nodeID1, streamID2, position2, 2)
			require.NoError(t, err)
		}

		// test delete all for a node
		{
			queueItems, err := geDB.GetIncomplete(ctx, nodeID2, 10, 0)
			require.NoError(t, err)
			require.Len(t, queueItems, 2)

			err = geDB.DeleteTransferQueueItems(ctx, nodeID2)
			require.NoError(t, err)

			queueItems, err = geDB.GetIncomplete(ctx, nodeID2, 10, 0)
			require.NoError(t, err)
			require.Len(t, queueItems, 0)
		}

		// test increment order limit send count
		err := geDB.IncrementOrderLimitSendCount(ctx, nodeID1, streamID2, position2, 2)
		require.NoError(t, err)

		// get queue item for key2 since that still exists
		item, err := geDB.GetTransferQueueItem(ctx, nodeID1, streamID2, position2, 2)
		require.NoError(t, err)

		require.Equal(t, 1, item.OrderLimitSendCount)
	})
}

func BenchmarkEnqueue(b *testing.B) {
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		gracefulexitdb := db.GracefulExit()
		ctx := context.Background()

		b.Run("BatchUpdateStats-100", func(b *testing.B) {
			batch(ctx, b, gracefulexitdb, 100)
		})
		if !testing.Short() {
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
		}
	})
}
func batch(ctx context.Context, b *testing.B, db gracefulexit.DB, size int) {
	for i := 0; i < b.N; i++ {
		var transferQueueItems []gracefulexit.TransferQueueItem
		for j := 0; j < size; j++ {
			item := gracefulexit.TransferQueueItem{
				NodeID:          testrand.NodeID(),
				StreamID:        testrand.UUID(),
				Position:        metabase.SegmentPosition{},
				PieceNum:        0,
				DurabilityRatio: 1.0,
			}
			transferQueueItems = append(transferQueueItems, item)
		}
		batchSize := 1000
		err := db.Enqueue(ctx, transferQueueItems, batchSize)
		require.NoError(b, err)
	}
}
