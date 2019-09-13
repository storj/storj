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
		for _, data := range []struct {
			nodeID storj.NodeID
			updAmt int64
			incAmt int64
		}{
			{testrand.NodeID(), 1, 2},
			{testrand.NodeID(), 3, 4},
		} {
			err := geDB.CreateProgress(ctx, data.nodeID)
			require.NoError(t, err)

			progress, err := geDB.GetProgress(ctx, data.nodeID)
			require.NoError(t, err)
			require.Equal(t, int64(0), progress.BytesTransferred)

			err = geDB.UpdateProgress(ctx, data.nodeID, data.updAmt)
			require.NoError(t, err)

			progress, err = geDB.GetProgress(ctx, data.nodeID)
			require.NoError(t, err)
			require.Equal(t, data.updAmt, progress.BytesTransferred)

			err = geDB.IncrementProgressBytesTransferred(ctx, data.nodeID, data.incAmt)
			require.NoError(t, err)

			progress, err = geDB.GetProgress(ctx, data.nodeID)
			require.NoError(t, err)
			require.Equal(t, data.updAmt+data.incAmt, progress.BytesTransferred)
		}
	})
}

func TestTransferQueueItem(t *testing.T) {
	// test basic graceful exit transfer queue crud
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := context.Background()

		geDB := db.GracefulExit()

		nodeID := testrand.NodeID()
		path := testrand.Bytes(memory.B * 32)

		item := &gracefulexit.TransferQueueItem{
			NodeID:          nodeID,
			Path:            path,
			PieceNum:        1,
			DurabilityRatio: 1.1,
		}

		err := geDB.CreateTransferQueueItem(ctx, *item)
		require.NoError(t, err)

		item, err = geDB.GetTransferQueueItem(ctx, nodeID, path)
		require.NoError(t, err)

		item.DurabilityRatio = 1.2
		item.RequestedAt = time.Now()

		err = geDB.UpdateTransferQueueItem(ctx, *item)
		require.NoError(t, err)

		latestItem, err := geDB.GetTransferQueueItem(ctx, nodeID, path)
		require.NoError(t, err)
		require.Equal(t, item.DurabilityRatio, latestItem.DurabilityRatio)
		require.True(t, item.RequestedAt.Equal(latestItem.RequestedAt))

		err = geDB.DeleteTransferQueueItem(ctx, nodeID, path)
		require.NoError(t, err)

		_, err = geDB.GetTransferQueueItem(ctx, nodeID, path)
		require.Error(t, err)
	})
}
