// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
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
		disableAsOfSystemTime := time.Second * 0
		finishedNodes, err := geDB.GetFinishedExitNodes(ctx, threeDays, disableAsOfSystemTime)
		require.NoError(t, err)
		require.Len(t, finishedNodes, 3)

		finishedNodes, err = geDB.GetFinishedExitNodes(ctx, currentTime, disableAsOfSystemTime)
		require.NoError(t, err)
		require.Len(t, finishedNodes, 6)

		count, err := geDB.DeleteFinishedExitProgress(ctx, threeDays, disableAsOfSystemTime)
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

func TestGracefulExit_HandleAsOfSystemTimeBadInput(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		gracefulexitDB := planet.Satellites[0].DB.GracefulExit()
		now := time.Now().UTC()
		// explicitly set as of system time to invalid time values and run queries to ensure queries don't break
		badTime1 := -1 * time.Nanosecond
		_, err := gracefulexitDB.CountFinishedTransferQueueItemsByNode(ctx, now, badTime1)
		require.NoError(t, err)
		badTime2 := 1 * time.Second
		_, err = gracefulexitDB.DeleteFinishedExitProgress(ctx, now, badTime2)
		require.NoError(t, err)
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

		// Mark some of the storagenodes as successful exit
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

		// Mark some of the storagenodes as failed exit
		nodeFailed1 := planet.StorageNodes[4]
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeFailed1.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-28 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-20 * time.Minute),
			ExitSuccess:         false,
		})
		require.NoError(t, err)

		nodeFailed2 := planet.StorageNodes[5]
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeFailed2.ID(),
			ExitInitiatedAt:     currentTime.Add(-time.Hour),
			ExitLoopCompletedAt: currentTime.Add(-17 * time.Minute),
			ExitFinishedAt:      currentTime.Add(-15 * time.Minute),
			ExitSuccess:         false,
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

		queueItemsPerNode := 500
		// Add some items to the transfer queue for the exited nodes
		queueItems, nodesItems := generateTransferQueueItems(t, queueItemsPerNode, []*testplanet.StorageNode{
			nodeSuccessful1, nodeSuccessful2, nodeSuccessful3, nodeFailed1, nodeFailed2,
		})

		gracefulExitDB := planet.Satellites[0].DB.GracefulExit()
		batchSize := 1000

		err = gracefulExitDB.Enqueue(ctx, queueItems, batchSize)
		require.NoError(t, err)

		asOfSystemTime := -1 * time.Microsecond
		// Count nodes exited before 15 minutes ago
		nodes, err := gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(-15*time.Minute), asOfSystemTime)
		require.NoError(t, err)
		require.Len(t, nodes, 3, "invalid number of nodes which have exited 15 minutes ago")

		for id, n := range nodes {
			assert.EqualValues(t, nodesItems[id], n, "unexpected number of items")
		}

		// Count nodes exited before 4 minutes ago
		nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(-4*time.Minute), asOfSystemTime)
		require.NoError(t, err)
		require.Len(t, nodes, 5, "invalid number of nodes which have exited 4 minutes ago")

		for id, n := range nodes {
			assert.EqualValues(t, nodesItems[id], n, "unexpected number of items")
		}

		// Delete items of nodes exited before 15 minutes ago
		count, err := gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime.Add(-15*time.Minute), asOfSystemTime, batchSize)
		require.NoError(t, err)
		expectedNumDeletedItems := nodesItems[nodeSuccessful1.ID()] +
			nodesItems[nodeSuccessful2.ID()] +
			nodesItems[nodeFailed1.ID()]
		require.EqualValues(t, expectedNumDeletedItems, count, "invalid number of deleted items")

		// Check that only a few nodes have exited are left with items
		nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(time.Minute), asOfSystemTime)
		require.NoError(t, err)
		require.Len(t, nodes, 2, "invalid number of exited nodes with items")

		for id, n := range nodes {
			assert.EqualValues(t, nodesItems[id], n, "unexpected number of items")
			assert.NotEqual(t, nodeSuccessful1.ID(), id, "node shouldn't have items")
			assert.NotEqual(t, nodeSuccessful2.ID(), id, "node shouldn't have items")
			assert.NotEqual(t, nodeFailed1.ID(), id, "node shouldn't have items")
		}

		// Delete the rest of the nodes' items
		count, err = gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime.Add(time.Minute), asOfSystemTime, batchSize)
		require.NoError(t, err)
		expectedNumDeletedItems = nodesItems[nodeSuccessful3.ID()] + nodesItems[nodeFailed2.ID()]
		require.EqualValues(t, expectedNumDeletedItems, count, "invalid number of deleted items")

		// Check that there aren't more exited nodes with items
		nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime.Add(time.Minute), asOfSystemTime)
		require.NoError(t, err)
		require.Len(t, nodes, 0, "invalid number of exited nodes with items")
	})
}

func TestGracefulExit_CopiedObjects(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		testGETransferQueue := func(node storj.NodeID, segmentsToTransfer int) {
			_, err = planet.Satellites[0].Overlay.DB.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
				NodeID:          node,
				ExitInitiatedAt: time.Now().UTC(),
			})
			require.NoError(t, err)

			// run the satellite ranged loop to build the transfer queue.
			_, err = planet.Satellites[0].RangedLoop.RangedLoop.Service.RunOnce(ctx)
			require.NoError(t, err)

			// we should get two items from GE queue as we have remote segment and its copy
			items, err := planet.Satellites[0].DB.GracefulExit().GetIncomplete(ctx, node, 100, 0)
			require.NoError(t, err)
			require.Len(t, items, segmentsToTransfer)

			segments, err := planet.Satellites[0].Metabase.DB.TestingAllSegments(ctx)
			require.NoError(t, err)

			for _, segment := range segments {
				if !segment.Inline() {
					require.True(t, slices.ContainsFunc(items, func(item *gracefulexit.TransferQueueItem) bool {
						return item.StreamID == segment.StreamID
					}))
				}
			}
		}

		// upload inline and remote and make copies
		for _, size := range []memory.Size{memory.KiB, 10 * memory.KiB} {
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "my-bucket", "my-object-"+size.String(), testrand.Bytes(size))
			require.NoError(t, err)

			_, err = project.CopyObject(ctx, "my-bucket", "my-object-"+size.String(), "my-bucket", "my-object-"+size.String()+"-copy", nil)
			require.NoError(t, err)
		}

		testGETransferQueue(planet.StorageNodes[0].ID(), 2)

		// delete original objects
		for _, size := range []memory.Size{memory.KiB, 10 * memory.KiB} {
			_, err = project.DeleteObject(ctx, "my-bucket", "my-object-"+size.String())
			require.NoError(t, err)
		}

		testGETransferQueue(planet.StorageNodes[1].ID(), 1)
	})
}

// TestGracefulExit_Enqueue_And_DeleteAllFinishedTransferQueueItems_batch
// ensures that deletion works as expected using different batch sizes.
func TestGracefulExit_Enqueue_And_DeleteAllFinishedTransferQueueItems_batchsize(t *testing.T) {
	var testCases = []struct {
		name                      string
		batchSize                 int
		transferQueueItemsPerNode int
		numExitedNodes            int
	}{
		{"less than complete batch, odd batch", 333, 3, 30},
		{"less than complete batch, even batch", 8888, 222, 40},
		{"over complete batch, odd batch", 3000, 200, 25},
		{"over complete batch, even batch", 1000, 110, 10},
		{"exact batch, odd batch", 1125, 25, 45},
		{"exact batch, even batch", 7200, 1200, 6},
	}
	for _, tt := range testCases {
		tt := tt
		satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
			var (
				gracefulExitDB = db.GracefulExit()
				currentTime    = time.Now().UTC()
				batchSize      = tt.batchSize
				numItems       = tt.transferQueueItemsPerNode
				numExitedNodes = tt.numExitedNodes
			)

			exitedNodeIDs := generateExitedNodes(t, ctx, db, currentTime, numExitedNodes)
			queueItems := generateNTransferQueueItemsPerNode(t, numItems, exitedNodeIDs...)

			// Add some items to the transfer queue for the exited nodes.
			err := gracefulExitDB.Enqueue(ctx, queueItems, batchSize)
			require.NoError(t, err)

			disableAsOfSystemTime := time.Second * 0
			// Count exited nodes
			nodes, err := gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime, disableAsOfSystemTime)
			require.NoError(t, err)
			require.EqualValues(t, numExitedNodes, len(nodes), "invalid number of exited nodes")

			// Delete items of the exited nodes
			count, err := gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime, disableAsOfSystemTime, batchSize)
			require.NoError(t, err)
			require.EqualValues(t, len(queueItems), count, "invalid number of deleted items")

			// Count exited nodes. At this time there shouldn't be any exited node with
			// items in the queue
			nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime, disableAsOfSystemTime)
			require.NoError(t, err)
			require.EqualValues(t, 0, len(nodes), "invalid number of exited nodes")

			// Delete items of the exited nodes. At this time there shouldn't be any
			count, err = gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime.Add(-15*time.Minute), disableAsOfSystemTime, batchSize)
			require.NoError(t, err)
			require.Zero(t, count, "invalid number of deleted items")
		})
	}
}

func generateExitedNodes(t *testing.T, ctx *testcontext.Context, db satellite.DB, currentTime time.Time, numExitedNodes int) (exitedNodeIDs storj.NodeIDList) {
	const (
		addr    = "127.0.1.0:8080"
		lastNet = "127.0.0"
	)
	var (
		cache      = db.OverlayCache()
		nodeIDsMap = make(map[storj.NodeID]struct{})
	)
	for i := 0; i < numExitedNodes; i++ {
		nodeID := generateNodeIDFromPostiveInt(t, i)
		exitedNodeIDs = append(exitedNodeIDs, nodeID)
		if _, ok := nodeIDsMap[nodeID]; ok {
			t.Logf("this %v already exists\n", nodeID.Bytes())
		}
		nodeIDsMap[nodeID] = struct{}{}

		info := overlay.NodeCheckInInfo{
			NodeID:     nodeID,
			Address:    &pb.NodeAddress{Address: addr},
			LastIPPort: addr,
			LastNet:    lastNet,
			Version:    &pb.NodeVersion{Version: "v1.0.0"},
			Capacity:   &pb.NodeCapacity{},
			IsUp:       true,
		}
		err := cache.UpdateCheckIn(ctx, info, time.Now(), overlay.NodeSelectionConfig{})
		require.NoError(t, err)

		exitFinishedAt := currentTime.Add(time.Duration(-(rand.Int63n(15) + 1) * int64(time.Minute)))
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              nodeID,
			ExitInitiatedAt:     exitFinishedAt.Add(-30 * time.Minute),
			ExitLoopCompletedAt: exitFinishedAt.Add(-20 * time.Minute),
			ExitFinishedAt:      exitFinishedAt,
			ExitSuccess:         true,
		})
		require.NoError(t, err)
	}
	require.Equal(t, numExitedNodes, len(nodeIDsMap), "map")
	return exitedNodeIDs
}

// TestGracefulExit_DeleteAllFinishedTransferQueueItems_batch verifies that
// the CRDB batch logic for delete all the transfer queue items of exited nodes
// works as expected.
func TestGracefulExit_DeleteAllFinishedTransferQueueItems_batch(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		const (
			addr    = "127.0.1.0:8080"
			lastNet = "127.0.0"
		)
		var (
			numNonExitedNodes = rand.Intn(20) + 1
			numExitedNodes    = rand.Intn(10) + 20
			cache             = db.OverlayCache()
		)

		for i := 0; i < numNonExitedNodes; i++ {
			info := overlay.NodeCheckInInfo{
				NodeID:     generateNodeIDFromPostiveInt(t, i),
				Address:    &pb.NodeAddress{Address: addr},
				LastIPPort: addr,
				LastNet:    lastNet,
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
				Capacity:   &pb.NodeCapacity{},
				IsUp:       true,
			}
			err := cache.UpdateCheckIn(ctx, info, time.Now(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
		}

		var (
			currentTime   = time.Now().UTC()
			exitedNodeIDs = make([]storj.NodeID, 0, numNonExitedNodes)
			nodeIDsMap    = make(map[storj.NodeID]struct{})
		)
		for i := numNonExitedNodes; i < (numNonExitedNodes + numExitedNodes); i++ {
			nodeID := generateNodeIDFromPostiveInt(t, i)
			exitedNodeIDs = append(exitedNodeIDs, nodeID)
			if _, ok := nodeIDsMap[nodeID]; ok {
				t.Logf("this %v already exists\n", nodeID.Bytes())
			}
			nodeIDsMap[nodeID] = struct{}{}

			info := overlay.NodeCheckInInfo{
				NodeID:     nodeID,
				Address:    &pb.NodeAddress{Address: addr},
				LastIPPort: addr,
				LastNet:    lastNet,
				Version:    &pb.NodeVersion{Version: "v1.0.0"},
				Capacity:   &pb.NodeCapacity{},
				IsUp:       true,
			}
			err := cache.UpdateCheckIn(ctx, info, time.Now(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			exitFinishedAt := currentTime.Add(time.Duration(-(rand.Int63n(15) + 1) * int64(time.Minute)))
			_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
				NodeID:              nodeID,
				ExitInitiatedAt:     exitFinishedAt.Add(-30 * time.Minute),
				ExitLoopCompletedAt: exitFinishedAt.Add(-20 * time.Minute),
				ExitFinishedAt:      exitFinishedAt,
				ExitSuccess:         true,
			})
			require.NoError(t, err)
		}

		require.Equal(t, numExitedNodes, len(nodeIDsMap), "map")

		gracefulExitDB := db.GracefulExit()
		batchSize := 1000

		queueItems := generateNTransferQueueItemsPerNode(t, 25, exitedNodeIDs...)
		// Add some items to the transfer queue for the exited nodes.
		err := gracefulExitDB.Enqueue(ctx, queueItems, batchSize)
		require.NoError(t, err)

		disableAsOfSystemTime := time.Second * 0
		// Count exited nodes
		nodes, err := gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime, disableAsOfSystemTime)
		require.NoError(t, err)
		require.EqualValues(t, numExitedNodes, len(nodes), "invalid number of exited nodes")

		// Delete items of the exited nodes
		count, err := gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime, disableAsOfSystemTime, batchSize)
		require.NoError(t, err)
		require.EqualValues(t, len(queueItems), count, "invalid number of deleted items")

		// Count exited nodes. At this time there shouldn't be any exited node with
		// items in the queue
		nodes, err = gracefulExitDB.CountFinishedTransferQueueItemsByNode(ctx, currentTime, disableAsOfSystemTime)
		require.NoError(t, err)
		require.EqualValues(t, 0, len(nodes), "invalid number of exited nodes")

		// Delete items of the exited nodes. At this time there shouldn't be any
		count, err = gracefulExitDB.DeleteAllFinishedTransferQueueItems(ctx, currentTime.Add(-15*time.Minute), disableAsOfSystemTime, batchSize)
		require.NoError(t, err)
		require.Zero(t, count, "invalid number of deleted items")
	})
}

// generateTransferQueueItems generates a random number of transfer queue items,
// between 10 and 120, for each passed node.
func generateTransferQueueItems(t *testing.T, itemsPerNode int, nodes []*testplanet.StorageNode) ([]gracefulexit.TransferQueueItem, map[storj.NodeID]int64) {
	nodeIDs := make([]storj.NodeID, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID()
	}

	items := generateNTransferQueueItemsPerNode(t, itemsPerNode, nodeIDs...)

	nodesItems := make(map[storj.NodeID]int64, len(nodes))
	for _, item := range items {
		nodesItems[item.NodeID]++
	}

	return items, nodesItems
}

// generateNTransferQueueItemsPerNode generates n queue items for each nodeID.
func generateNTransferQueueItemsPerNode(t *testing.T, n int, nodeIDs ...storj.NodeID) []gracefulexit.TransferQueueItem {
	items := make([]gracefulexit.TransferQueueItem, 0)
	for _, nodeID := range nodeIDs {
		for i := 0; i < n; i++ {
			items = append(items, gracefulexit.TransferQueueItem{
				NodeID:   nodeID,
				StreamID: testrand.UUID(),
				Position: metabase.SegmentPositionFromEncoded(rand.Uint64()),
				PieceNum: rand.Int31(),
			})
		}
	}
	return items
}

// generateNodeIDFromPostiveInt generates a specific node ID for val; each val
// value produces a different node ID.
func generateNodeIDFromPostiveInt(t *testing.T, val int) storj.NodeID {
	t.Helper()

	if val < 0 {
		t.Fatal("cannot generate a node from a negative integer")
	}

	nodeID := storj.NodeID{}
	idx := 0
	for {
		m := val & 255
		nodeID[idx] = byte(m)

		q := val >> 8
		if q == 0 {
			break
		}
		if q < 256 {
			nodeID[idx+1] = byte(q)
			break
		}

		val = q
		idx++
	}

	return nodeID
}
