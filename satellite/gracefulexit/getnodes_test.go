// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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
		for _, id := range nodes {
			require.True(t, exiting[id])
		}

		nodes, err = cache.GetExitingNodesLoopIncomplete(ctx)
		require.NoError(t, err)
		require.Len(t, nodes, exitingLoopIncompleteCount)
		for _, id := range nodes {
			require.True(t, exitingLoopIncomplete[id])
		}
	})
}
