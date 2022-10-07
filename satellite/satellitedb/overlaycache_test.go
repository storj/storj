// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
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
