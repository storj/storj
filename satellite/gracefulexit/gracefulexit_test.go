// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
)

func TestGracefulexitDB_DeleteFinishedExitProgress(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		geDB := planet.Satellites[0].DB.GracefulExit()

		days := 6
		currentTime := time.Now().UTC()
		// Set timestamp back by the number of days we want to save
		timestamp := currentTime.AddDate(0, 0, -days).Truncate(time.Millisecond)

		for i := 0; i < days; i++ {
			nodeID := planet.StorageNodes[i].ID()
			err := geDB.IncrementProgress(ctx, nodeID, 100, 100, 100)
			require.NoError(t, err)

			_, err = satellite.Overlay.DB.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
				NodeID:         nodeID,
				ExitFinishedAt: timestamp,
			})
			require.NoError(t, err)

			// Advance time by 24 hours
			timestamp = timestamp.Add(time.Hour * 24)
		}
		threeDays := currentTime.AddDate(0, 0, -days/2).Add(-time.Millisecond)
		finishedNodes, err := geDB.GetFinishedExitNodes(ctx, threeDays)
		require.NoError(t, err)
		require.Len(t, finishedNodes, 3)

		finishedNodes, err = geDB.GetFinishedExitNodes(ctx, currentTime)
		require.NoError(t, err)
		require.Len(t, finishedNodes, 6)

		count, err := geDB.DeleteFinishedExitProgress(ctx, threeDays)
		require.NoError(t, err)
		require.EqualValues(t, 3, count)

		// Check that first three nodes were removed from exit progress table
		for i, node := range planet.StorageNodes {
			progress, err := geDB.GetProgress(ctx, node.ID())
			if i < 3 {
				require.True(t, gracefulexit.ErrNodeNotFound.Has(err))
				require.Nil(t, progress)
			} else {
				require.NoError(t, err)
			}
		}
	})
}

func TestGracefulExit_DeleteAllFinishedTransferQueueItems(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 7,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var (
			cache       = planet.Satellites[0].DB.OverlayCache()
			currentTime = time.Now().UTC()
		)

		// mark some of the storagenodes as successful exit
		nodeSuccessful1 := planet.StorageNodes[1]
		_, err := cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeSuccessful1.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-30 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-25 * time.Minute),
			ExitSuccess:         true,
		})
		require.NoError(t, err)

		nodeSuccessful2 := planet.StorageNodes[2]
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeSuccessful2.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-17 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-16 * time.Minute),
			ExitSuccess:         true,
		})
		require.NoError(t, err)

		nodeSuccessful3 := planet.StorageNodes[3]
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeSuccessful3.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-9 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-6 * time.Minute),
			ExitSuccess:         true,
		})
		require.NoError(t, err)

		// mark some of the storagenodes as failed exit
		nodeFailed1 := planet.StorageNodes[4]
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeFailed1.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-28 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-20 * time.Minute),
			ExitSuccess:         true,
		})
		require.NoError(t, err)

		nodeFailed2 := planet.StorageNodes[5]
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeFailed2.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-17 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-15 * time.Minute),
			ExitSuccess:         true,
		})
		require.NoError(t, err)

		nodeWithoutItems := planet.StorageNodes[6]
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeWithoutItems.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-35 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-32 * time.Minute),
			ExitSuccess:         false,
		})
		require.NoError(t, err)

		// add some items to the transfer queue for the exited nodes
		queueItems, nodesItems := generateTransferQueueItems(t, []*testplanet.StorageNode{
			nodeSuccessful1, nodeSuccessful2, nodeSuccessful3, nodeFailed1, nodeFailed2,
		})

		gracefulExitDB := planet.Satellites[0].DB.GracefulExit()
		err = gracefulExitDB.Enqueue(ctx, queueItems)
		require.NoError(t, err)

		// count nodes exited before 15 minutes ago
		nodes, err := gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(-15*time.Minute))
		require.NoError(t, err)
		require.Len(t, nodes, 3, "invalid number of nodes which have exited 15 minutes ago")

		for id, n := range nodes {
			assert.EqualValues(t, nodesItems[id], n, "unexpected number of items")
		}

		// count nodes exited before 4 minutes ago
		nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(-4*time.Minute))
		require.NoError(t, err)
		require.Len(t, nodes, 5, "invalid number of nodes which have exited 4 minutes ago")

		for id, n := range nodes {
			assert.EqualValues(t, nodesItems[id], n, "unexpected number of items")
		}

		// delete items of nodes exited before 15 minutes ago
		count, err := gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime.Add(-15*time.Minute))
		require.NoError(t, err)
		expectedNumDeletedItems := nodesItems[nodeSuccessful1.ID()] +
			nodesItems[nodeSuccessful2.ID()] +
			nodesItems[nodeFailed1.ID()]
		require.EqualValues(t, expectedNumDeletedItems, count, "invalid number of delet items")

		// check that only a few nodes have exited are left with items
		nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, nodes, 2, "invalid number of exited nodes with items")

		for id, n := range nodes {
			assert.EqualValues(t, nodesItems[id], n, "unexpected number of items")
			assert.NotEqual(t, nodeSuccessful1.ID(), id, "node shouldn't have items")
			assert.NotEqual(t, nodeSuccessful2.ID(), id, "node shouldn't have items")
			assert.NotEqual(t, nodeFailed1.ID(), id, "node shouldn't have items")
		}

		// delete items of there rest exited nodes
		count, err = gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime.Add(time.Minute))
		require.NoError(t, err)
		expectedNumDeletedItems = nodesItems[nodeSuccessful3.ID()] + nodesItems[nodeFailed2.ID()]
		require.EqualValues(t, expectedNumDeletedItems, count, "invalid number of delet items")

		// check that there aren't more exited nodes with items
		nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, nodes, 0, "invalid number of exited nodes with items")
	})
}

func generateTransferQueueItems(t *testing.T, nodes []*testplanet.StorageNode) ([]gracefulexit.TransferQueueItem, map[storj.NodeID]int64) {
	getNodeID := func() storj.NodeID {
		n := rand.Intn(len(nodes))
		return nodes[n].ID()
	}

	var (
		items      = make([]gracefulexit.TransferQueueItem, rand.Intn(100)+10)
		nodesItems = make(map[storj.NodeID]int64, len(items))
	)
	for i, item := range items {
		item.NodeID = getNodeID()
		item.Key = metabase.SegmentKey{byte(i)}
		item.PieceNum = int32(i + 1)
		items[i] = item
		nodesItems[item.NodeID]++
	}

	return items, nodesItems
}
