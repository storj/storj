// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProgress(t *testing.T) {
	// test basic graceful exit progress crud
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

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

func TestTransferQueueItem(t *testing.T) {
	// test basic graceful exit transfer queue crud
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		geDB := db.GracefulExit()

		nodeID1 := testrand.NodeID()
		nodeID2 := testrand.NodeID()
		path1 := testrand.Bytes(memory.B * 32)
		path2 := testrand.Bytes(memory.B * 32)
		// root piece IDs for path 1 and 2
		rootPieceID1 := testrand.PieceID()
		rootPieceID2 := testrand.PieceID()
		items := []gracefulexit.TransferQueueItem{
			{
				NodeID:          nodeID1,
				Path:            path1,
				PieceNum:        1,
				RootPieceID:     rootPieceID1,
				DurabilityRatio: 0.9,
			},
			{
				NodeID:          nodeID1,
				Path:            path2,
				PieceNum:        2,
				RootPieceID:     rootPieceID2,
				DurabilityRatio: 1.1,
			},
			{
				NodeID:          nodeID2,
				Path:            path1,
				PieceNum:        2,
				RootPieceID:     rootPieceID1,
				DurabilityRatio: 0.9,
			},
			{
				NodeID:          nodeID2,
				Path:            path2,
				PieceNum:        1,
				RootPieceID:     rootPieceID2,
				DurabilityRatio: 1.1,
			},
		}

		// test basic create, update, get delete
		{
			err := geDB.Enqueue(ctx, items)
			require.NoError(t, err)

			for _, tqi := range items {
				item, err := geDB.GetTransferQueueItem(ctx, tqi.NodeID, tqi.Path, tqi.PieceNum)
				require.NoError(t, err)
				require.Equal(t, tqi.RootPieceID, item.RootPieceID)
				require.Equal(t, tqi.DurabilityRatio, item.DurabilityRatio)

				now := time.Now().UTC()
				item.DurabilityRatio = 1.2
				item.RequestedAt = &now

				err = geDB.UpdateTransferQueueItem(ctx, *item)
				require.NoError(t, err)

				latestItem, err := geDB.GetTransferQueueItem(ctx, tqi.NodeID, tqi.Path, tqi.PieceNum)
				require.NoError(t, err)

				require.Equal(t, item.RootPieceID, latestItem.RootPieceID)
				require.Equal(t, item.DurabilityRatio, latestItem.DurabilityRatio)
				require.True(t, item.RequestedAt.Truncate(time.Millisecond).Equal(latestItem.RequestedAt.Truncate(time.Millisecond)))
			}

			queueItems, err := geDB.GetIncomplete(ctx, nodeID1, 10, 0)
			require.NoError(t, err)
			require.Len(t, queueItems, 2)
		}
		// mark the first item finished and test that only 1 item gets returned from the GetIncomplete
		{
			item, err := geDB.GetTransferQueueItem(ctx, nodeID1, path1, 1)
			require.NoError(t, err)

			now := time.Now().UTC()
			item.FinishedAt = &now

			err = geDB.UpdateTransferQueueItem(ctx, *item)
			require.NoError(t, err)

			queueItems, err := geDB.GetIncomplete(ctx, nodeID1, 10, 0)
			require.NoError(t, err)
			require.Len(t, queueItems, 1)
			for _, queueItem := range queueItems {
				require.Equal(t, nodeID1, queueItem.NodeID)
				require.Equal(t, path2, queueItem.Path)
			}
		}

		// test delete finished queue items. Only path1 should be removed
		{
			err := geDB.DeleteFinishedTransferQueueItems(ctx, nodeID1)
			require.NoError(t, err)

			// path1 should no longer exist for nodeID1
			_, err = geDB.GetTransferQueueItem(ctx, nodeID1, path1, 1)
			require.Error(t, err)

			// path2 should still exist for nodeID1
			_, err = geDB.GetTransferQueueItem(ctx, nodeID1, path2, 2)
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
		err := geDB.IncrementOrderLimitSendCount(ctx, nodeID1, path2, 2)
		require.NoError(t, err)

		// get queue item for path2 since that still exists
		item, err := geDB.GetTransferQueueItem(ctx, nodeID1, path2, 2)
		require.NoError(t, err)

		require.Equal(t, 1, item.OrderLimitSendCount)
	})
}
