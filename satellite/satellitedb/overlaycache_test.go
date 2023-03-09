// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/private/version"
	"storj.io/storj/private/teststorj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetOfflineNodesForEmail(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		selectionCfg := overlay.NodeSelectionConfig{
			OnlineWindow: 4 * time.Hour,
		}

		offlineID := teststorj.NodeIDFromString("offlineNode")
		onlineID := teststorj.NodeIDFromString("onlineNode")
		disqualifiedID := teststorj.NodeIDFromString("dqNode")
		exitedID := teststorj.NodeIDFromString("exitedNode")
		offlineNoEmailID := teststorj.NodeIDFromString("noEmail")

		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "offline@storj.test",
			},
		}

		now := time.Now()

		// offline node should be selected
		checkInInfo.NodeID = offlineID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), selectionCfg))

		// online node should not be selected
		checkInInfo.NodeID = onlineID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))

		// disqualified node should not be selected
		checkInInfo.NodeID = disqualifiedID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), selectionCfg))
		_, err := cache.DisqualifyNode(ctx, disqualifiedID, now, overlay.DisqualificationReasonUnknown)
		require.NoError(t, err)

		// exited node should not be selected
		checkInInfo.NodeID = exitedID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), selectionCfg))
		_, err = cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
			NodeID:              exitedID,
			ExitInitiatedAt:     now,
			ExitLoopCompletedAt: now,
			ExitFinishedAt:      now,
			ExitSuccess:         true,
		})
		require.NoError(t, err)

		// node with no email should not be selected
		checkInInfo.NodeID = offlineNoEmailID
		checkInInfo.Operator.Email = ""
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-24*time.Hour), selectionCfg))

		nodes, err := cache.GetOfflineNodesForEmail(ctx, selectionCfg.OnlineWindow, 72*time.Hour, 24*time.Hour, 10)
		require.NoError(t, err)
		require.Equal(t, 1, len(nodes))
		require.NotEmpty(t, nodes[offlineID])

		// test cutoff causes node to not be selected
		nodes, err = cache.GetOfflineNodesForEmail(ctx, selectionCfg.OnlineWindow, time.Second, 24*time.Hour, 10)
		require.NoError(t, err)
		require.Empty(t, nodes)
	})
}

func TestUpdateLastOfflineEmail(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		selectionCfg := overlay.NodeSelectionConfig{
			OnlineWindow: 4 * time.Hour,
		}

		nodeID0 := teststorj.NodeIDFromString("testnode0")
		nodeID1 := teststorj.NodeIDFromString("testnode1")

		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "test@storj.test",
			},
		}

		now := time.Now()
		checkInInfo.NodeID = nodeID0
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))
		checkInInfo.NodeID = nodeID1
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))
		require.NoError(t, cache.UpdateLastOfflineEmail(ctx, []storj.NodeID{nodeID0, nodeID1}, now))

		node0, err := cache.Get(ctx, nodeID0)
		require.NoError(t, err)
		require.Equal(t, now.Truncate(time.Second), node0.LastOfflineEmail.Truncate(time.Second))

		node1, err := cache.Get(ctx, nodeID1)
		require.NoError(t, err)
		require.Equal(t, now.Truncate(time.Second), node1.LastOfflineEmail.Truncate(time.Second))
	})
}

func TestSetNodeContained(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		nodeID := testrand.NodeID()
		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "offline@storj.test",
			},
		}

		now := time.Now()

		// offline node should be selected
		checkInInfo.NodeID = nodeID
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now.Add(-24*time.Hour), overlay.NodeSelectionConfig{}))

		cacheInfo, err := cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.False(t, cacheInfo.Contained)

		err = cache.SetNodeContained(ctx, nodeID, true)
		require.NoError(t, err)

		cacheInfo, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.True(t, cacheInfo.Contained)

		err = cache.SetNodeContained(ctx, nodeID, false)
		require.NoError(t, err)

		cacheInfo, err = cache.Get(ctx, nodeID)
		require.NoError(t, err)
		require.False(t, cacheInfo.Contained)
	})
}

func TestUpdateCheckInDirectUpdate(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()
		db.OverlayCache()
		selectionCfg := overlay.NodeSelectionConfig{
			OnlineWindow: 4 * time.Hour,
		}
		nodeID := teststorj.NodeIDFromString("testnode0")
		checkInInfo := overlay.NodeCheckInInfo{
			IsUp: true,
			Address: &pb.NodeAddress{
				Address: "1.2.3.4",
			},
			Version: &pb.NodeVersion{
				Version:    "v0.0.0",
				CommitHash: "",
				Timestamp:  time.Time{},
				Release:    false,
			},
			Operator: &pb.NodeOperator{
				Email: "test@storj.test",
			},
		}
		now := time.Now().UTC()
		checkInInfo.NodeID = nodeID
		semVer, err := version.NewSemVer(checkInInfo.Version.Version)
		require.NoError(t, err)
		// node unknown - should not be updated by updateCheckInDirectUpdate
		updated, err := cache.TestUpdateCheckInDirectUpdate(ctx, checkInInfo, now, semVer, "encodedwalletfeature")
		require.NoError(t, err)
		require.False(t, updated)
		require.NoError(t, cache.UpdateCheckIn(ctx, checkInInfo, now, selectionCfg))
		updated, err = cache.TestUpdateCheckInDirectUpdate(ctx, checkInInfo, now.Add(6*time.Hour), semVer, "encodedwalletfeature")
		require.NoError(t, err)
		require.True(t, updated)
	})
}

func TestSetAllContainedNodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		node1 := testrand.NodeID()
		node2 := testrand.NodeID()
		node3 := testrand.NodeID()

		// put nodes with these IDs in the db
		for _, n := range []storj.NodeID{node1, node2, node3} {
			checkInInfo := overlay.NodeCheckInInfo{
				IsUp:    true,
				Address: &pb.NodeAddress{Address: "1.2.3.4"},
				Version: &pb.NodeVersion{Version: "v0.0.0"},
				NodeID:  n,
			}
			err := cache.UpdateCheckIn(ctx, checkInInfo, time.Now().UTC(), overlay.NodeSelectionConfig{})
			require.NoError(t, err)
		}
		// none of them should be contained
		assertContained(ctx, t, cache, node1, false, node2, false, node3, false)

		// Set node2 (only) to be contained
		err := cache.SetAllContainedNodes(ctx, []storj.NodeID{node2})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, false, node2, true, node3, false)

		// Set node1 and node3 (only) to be contained
		err = cache.SetAllContainedNodes(ctx, []storj.NodeID{node1, node3})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, true, node2, false, node3, true)

		// Set node1 (only) to be contained
		err = cache.SetAllContainedNodes(ctx, []storj.NodeID{node1})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, true, node2, false, node3, false)

		// Set no nodes to be contained
		err = cache.SetAllContainedNodes(ctx, []storj.NodeID{})
		require.NoError(t, err)
		assertContained(ctx, t, cache, node1, false, node2, false, node3, false)
	})
}

func assertContained(ctx context.Context, t testing.TB, cache overlay.DB, args ...interface{}) {
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
