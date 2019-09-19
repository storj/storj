// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestSatelliteDBSetup(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		testGetExitingNodes(ctx, t, db.OverlayCache())
	})
}

func testGetExitingNodes(ctx context.Context, t *testing.T, cache overlay.DB) {
	for _, tt := range []struct {
		numNodesToExit       int
		nodesTotal           int
		expectedExitingNodes int
	}{
		{2, 2, 2},
	} {
		for i := 0; i < tt.nodesTotal; i++ {
			newID := testrand.NodeID()
			// add nodes to cache
			err := cache.UpdateAddress(ctx, &pb.Node{Id: newID}, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			var (
				initiatedAt         *time.Time = nil
				completedAt         *time.Time = nil
				finishedAt          *time.Time = nil
				updateInitiated                = false
				updateLoopCompleted            = false
				updateFinished                 = false
			)

			// set some nodes to have an exiting status
			if i < tt.numNodesToExit {
				timestamp := time.Now().UTC()
				initiatedAt = &timestamp
				completedAt = nil
				updateInitiated = true
				updateLoopCompleted = true
			}

			req := &overlay.ExitStatusRequest{
				NodeID:              newID,
				ExitInitiatedAt:     initiatedAt,
				ExitLoopCompletedAt: completedAt,
				ExitFinishedAt:      finishedAt,
				UpdateInitiated:     updateInitiated,
				UpdateLoopCompleted: updateLoopCompleted,
				UpdateFinished:      updateFinished,
			}

			_, err = cache.UpdateExitStatus(ctx, req)
			require.NoError(t, err)
		}

		nodes, err := cache.GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, nodes, tt.expectedExitingNodes)
	}

}
