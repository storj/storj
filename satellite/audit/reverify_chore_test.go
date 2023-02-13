// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/overlay"
)

func TestContainmentSyncChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 3,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		reverifyQueue := satellite.Audit.ReverifyQueue
		cache := satellite.Overlay.DB
		syncChore := satellite.Audit.ContainmentSyncChore
		syncChore.Loop.Pause()

		node1 := planet.StorageNodes[0].ID()
		node2 := planet.StorageNodes[1].ID()
		node3 := planet.StorageNodes[2].ID()

		// no nodes should be in the reverify queue
		requireInReverifyQueue(ctx, t, reverifyQueue)
		// and none should be marked contained in the overlay
		requireContainedStatus(ctx, t, cache, node1, false, node2, false, node3, false)

		// set node1 contained in the overlay, but node2 contained in the reverify queue
		err := cache.SetNodeContained(ctx, node1, true)
		require.NoError(t, err)
		node2Piece := &audit.PieceLocator{StreamID: testrand.UUID(), NodeID: node2}
		err = reverifyQueue.Insert(ctx, node2Piece)
		require.NoError(t, err)
		requireInReverifyQueue(ctx, t, reverifyQueue, node2)
		requireContainedStatus(ctx, t, cache, node1, true, node2, false, node3, false)

		// run the chore to synchronize
		syncChore.Loop.TriggerWait()

		// there should only be node2 in both places now
		requireInReverifyQueue(ctx, t, reverifyQueue, node2)
		requireContainedStatus(ctx, t, cache, node1, false, node2, true, node3, false)

		// now get node3 in the reverify queue as well
		node3Piece := &audit.PieceLocator{StreamID: testrand.UUID(), NodeID: node3}
		err = reverifyQueue.Insert(ctx, node3Piece)
		require.NoError(t, err)
		requireInReverifyQueue(ctx, t, reverifyQueue, node2, node3)
		requireContainedStatus(ctx, t, cache, node1, false, node2, true, node3, false)

		// run the chore to synchronize
		syncChore.Loop.TriggerWait()

		// nodes 2 and 3 should both be contained in both places now
		requireInReverifyQueue(ctx, t, reverifyQueue, node2, node3)
		requireContainedStatus(ctx, t, cache, node1, false, node2, true, node3, true)

		// remove both node2 and node3 from reverify queue
		wasDeleted, err := reverifyQueue.Remove(ctx, node2Piece)
		require.NoError(t, err)
		require.True(t, wasDeleted)
		wasDeleted, err = reverifyQueue.Remove(ctx, node3Piece)
		require.NoError(t, err)
		require.True(t, wasDeleted)
		requireInReverifyQueue(ctx, t, reverifyQueue)
		requireContainedStatus(ctx, t, cache, node1, false, node2, true, node3, true)

		// run the chore to synchronize
		syncChore.Loop.TriggerWait()

		// nothing should be contained in either place now
		requireInReverifyQueue(ctx, t, reverifyQueue)
		requireContainedStatus(ctx, t, cache, node1, false, node2, false, node3, false)
	})
}

func requireInReverifyQueue(ctx context.Context, t testing.TB, reverifyQueue audit.ReverifyQueue, expectedNodes ...storj.NodeID) {
	nodesInReverifyQueue, err := reverifyQueue.GetAllContainedNodes(ctx)
	require.NoError(t, err)

	sort.Slice(nodesInReverifyQueue, func(i, j int) bool {
		return nodesInReverifyQueue[i].Compare(nodesInReverifyQueue[j]) < 0
	})
	sort.Slice(nodesInReverifyQueue, func(i, j int) bool {
		return expectedNodes[i].Compare(expectedNodes[j]) < 0
	})
	require.Equal(t, expectedNodes, nodesInReverifyQueue)
}

func requireContainedStatus(ctx context.Context, t testing.TB, cache overlay.DB, args ...interface{}) {
	require.Equal(t, 0, len(args)%2, "must be given an even number of args")
	for n := 0; n < len(args); n += 2 {
		nodeID := args[n].(storj.NodeID)
		expectedContainment := args[n+1].(bool)
		nodeInDB, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.Equalf(t, expectedContainment, nodeInDB.Contained,
			"Expected nodeID %v (args[%d]) contained = %v, but got %v",
			nodeID, n, expectedContainment, nodeInDB.Contained)
	}
}
