// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetExitingNodes(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		cache := db.OverlayCache()
		exiting := make(map[storj.NodeID]bool)
		exitingCount := 0
		exitingLoopIncomplete := make(map[storj.NodeID]bool)
		exitingLoopIncompleteCount := 0

		testData := []struct {
			nodeID                  storj.NodeID
			initiatedAt             time.Time
			completedAt             time.Time
			finishedAt              time.Time
			isExiting               bool
			isExitingLoopIncomplete bool
		}{
			{testrand.NodeID(), time.Time{}, time.Time{}, time.Time{}, false, false},
			{testrand.NodeID(), time.Now(), time.Time{}, time.Time{}, true, true},
			{testrand.NodeID(), time.Now(), time.Now(), time.Time{}, true, false},
			{testrand.NodeID(), time.Now(), time.Now(), time.Now(), false, false},
			{testrand.NodeID(), time.Now(), time.Time{}, time.Now(), false, false},
		}

		for _, data := range testData {
			err := cache.UpdateAddress(ctx, &pb.Node{Id: data.nodeID}, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			req := &overlay.ExitStatusRequest{
				NodeID:              data.nodeID,
				ExitInitiatedAt:     data.initiatedAt,
				ExitLoopCompletedAt: data.completedAt,
				ExitFinishedAt:      data.finishedAt,
			}
			_, err = cache.UpdateExitStatus(ctx, req)
			require.NoError(t, err)

			if data.isExiting {
				exitingCount++
				exiting[data.nodeID] = true
			}
			if data.isExitingLoopIncomplete {
				exitingLoopIncompleteCount++
				exitingLoopIncomplete[data.nodeID] = true
			}
		}

		nodes, err := cache.GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, nodes, exitingCount)
		for _, node := range nodes {
			require.True(t, exiting[node.NodeID])
		}

		nodes, err = cache.GetExitingNodes(ctx)
		require.NoError(t, err)
		exitingNodesLoopIncomplete := make(storj.NodeIDList, 0, len(nodes))
		for _, node := range nodes {
			if node.ExitLoopCompletedAt == nil {
				exitingNodesLoopIncomplete = append(exitingNodesLoopIncomplete, node.NodeID)
			}
		}
		require.Len(t, exitingNodesLoopIncomplete, exitingLoopIncompleteCount)
		for _, id := range exitingNodesLoopIncomplete {
			require.True(t, exitingLoopIncomplete[id])
		}
	})
}

func TestGetGracefulExitNodesByTimeframe(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		cache := db.OverlayCache()
		exitingToday := make(map[storj.NodeID]bool)
		exitingLastWeek := make(map[storj.NodeID]bool)
		exitedToday := make(map[storj.NodeID]bool)
		exitedLastWeek := make(map[storj.NodeID]bool)

		now := time.Now()
		lastWeek := time.Now().AddDate(0, 0, -7)

		testData := []struct {
			nodeID      storj.NodeID
			initiatedAt time.Time
			completedAt time.Time
			finishedAt  time.Time
		}{
			// exited today
			{testrand.NodeID(), now, now, now},
			// exited last week
			{testrand.NodeID(), lastWeek, lastWeek, lastWeek},
			// exiting today
			{testrand.NodeID(), now, now, time.Time{}},
			// exiting last week
			{testrand.NodeID(), lastWeek, lastWeek, time.Time{}},
			// not exiting
			{testrand.NodeID(), time.Time{}, time.Time{}, time.Time{}},
		}

		for _, data := range testData {
			err := cache.UpdateAddress(ctx, &pb.Node{Id: data.nodeID}, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			req := &overlay.ExitStatusRequest{
				NodeID:              data.nodeID,
				ExitInitiatedAt:     data.initiatedAt,
				ExitLoopCompletedAt: data.completedAt,
				ExitFinishedAt:      data.finishedAt,
			}
			_, err = cache.UpdateExitStatus(ctx, req)
			require.NoError(t, err)

			if !data.finishedAt.IsZero() {
				if data.finishedAt == now {
					exitedToday[data.nodeID] = true
				} else {
					exitedLastWeek[data.nodeID] = true
				}
			} else if !data.initiatedAt.IsZero() {
				if data.initiatedAt == now {
					exitingToday[data.nodeID] = true
				} else {
					exitingLastWeek[data.nodeID] = true
				}
			}

		}

		// test GetGracefulExitIncompleteByTimeFrame
		ids, err := cache.GetGracefulExitIncompleteByTimeFrame(ctx, lastWeek.Add(-24*time.Hour), lastWeek.Add(24*time.Hour))
		require.NoError(t, err)
		require.Len(t, ids, 1)
		for _, id := range ids {
			require.True(t, exitingLastWeek[id])
		}
		ids, err = cache.GetGracefulExitIncompleteByTimeFrame(ctx, now.Add(-24*time.Hour), now.Add(24*time.Hour))
		require.NoError(t, err)
		require.Len(t, ids, 1)
		for _, id := range ids {
			require.True(t, exitingToday[id])
		}
		ids, err = cache.GetGracefulExitIncompleteByTimeFrame(ctx, lastWeek.Add(-24*time.Hour), now.Add(24*time.Hour))
		require.NoError(t, err)
		require.Len(t, ids, 2)
		for _, id := range ids {
			require.True(t, exitingLastWeek[id] || exitingToday[id])
		}

		// test GetGracefulExitCompletedByTimeFrame
		ids, err = cache.GetGracefulExitCompletedByTimeFrame(ctx, lastWeek.Add(-24*time.Hour), lastWeek.Add(24*time.Hour))
		require.NoError(t, err)
		require.Len(t, ids, 1)
		for _, id := range ids {
			require.True(t, exitedLastWeek[id])
		}
		ids, err = cache.GetGracefulExitCompletedByTimeFrame(ctx, now.Add(-24*time.Hour), now.Add(24*time.Hour))
		require.NoError(t, err)
		require.Len(t, ids, 1)
		for _, id := range ids {
			require.True(t, exitedToday[id])
		}
		ids, err = cache.GetGracefulExitCompletedByTimeFrame(ctx, lastWeek.Add(-24*time.Hour), now.Add(24*time.Hour))
		require.NoError(t, err)
		require.Len(t, ids, 2)
		for _, id := range ids {
			require.True(t, exitedLastWeek[id] || exitedToday[id])
		}

	})
}
