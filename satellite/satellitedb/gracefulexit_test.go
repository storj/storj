// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestProgress(t *testing.T) {
	// test basic graceful exit progress crud
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := context.Background()

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
		ctx := context.Background()

		geDB := db.GracefulExit()

		nodeID1 := testrand.NodeID()
		nodeID2 := testrand.NodeID()
		path1 := testrand.Bytes(memory.B * 32)
		path2 := testrand.Bytes(memory.B * 32)
		items := []gracefulexit.TransferQueueItem{
			{
				NodeID:          nodeID1,
				Path:            path1,
				PieceNum:        1,
				DurabilityRatio: 0.9,
			},
			{
				NodeID:          nodeID1,
				Path:            path2,
				PieceNum:        2,
				DurabilityRatio: 1.1,
			},
			{
				NodeID:          nodeID2,
				Path:            path1,
				PieceNum:        2,
				DurabilityRatio: 0.9,
			},
			{
				NodeID:          nodeID2,
				Path:            path2,
				PieceNum:        1,
				DurabilityRatio: 1.1,
			},
		}

		// test basic create, update, get delete
		err := geDB.Enqueue(ctx, items)
		require.NoError(t, err)

		for _, tqi := range items {
			item, err := geDB.GetTransferQueueItem(ctx, tqi.NodeID, tqi.Path)
			require.NoError(t, err)

			item.DurabilityRatio = 1.2
			item.RequestedAt = time.Now()

			err = geDB.UpdateTransferQueueItem(ctx, *item)
			require.NoError(t, err)

			latestItem, err := geDB.GetTransferQueueItem(ctx, tqi.NodeID, tqi.Path)
			require.NoError(t, err)
			require.Equal(t, item.DurabilityRatio, latestItem.DurabilityRatio)
			require.True(t, item.RequestedAt.Truncate(time.Millisecond).Equal(latestItem.RequestedAt.Truncate(time.Millisecond)))
		}

		// mark the first item finished
		item, err := geDB.GetTransferQueueItem(ctx, nodeID1, path1)
		require.NoError(t, err)

		item.FinishedAt = time.Now()
		err = geDB.UpdateTransferQueueItem(ctx, *item)
		require.NoError(t, err)

		queueItems, err := geDB.GetIncomplete(ctx, nodeID1, 10, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(queueItems))
		for _, queueItem := range queueItems {
			require.Equal(t, nodeID1, queueItem.NodeID)
			require.NotEqual(t, path1, queueItem.Path)

			// test delete
			err := geDB.DeleteTransferQueueItem(ctx, queueItem.NodeID, queueItem.Path)
			require.NoError(t, err)
		}
		// test delete finished
		err = geDB.DeleteFinishedTransferQueueItems(ctx, nodeID1)
		require.NoError(t, err)

		_, err = geDB.GetTransferQueueItem(ctx, nodeID1, path1)
		require.Error(t, err)

		// test delete all for a node
		err = geDB.DeleteTransferQueueItems(ctx, nodeID2)
		require.NoError(t, err)

		queueItems, err = geDB.GetIncomplete(ctx, nodeID1, 10, 0)
		require.NoError(t, err)
		require.Equal(t, 0, len(queueItems))
	})
}
