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
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestGetExitingNodes(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			cache = db.OverlayCache()

			numTotalNodes   = 5
			numExitingNodes = 2
		)

		for i := 0; i < numTotalNodes; i++ {
			newID := testrand.NodeID()
			// add nodes to cache
			err := cache.UpdateAddress(ctx, &pb.Node{Id: newID}, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			var (
				initiatedAt         *time.Time
				completedAt         *time.Time
				finishedAt          *time.Time
				updateInitiated     = false
				updateLoopCompleted = false
				updateFinished      = false
			)

			// set some nodes to have an exiting status
			if i < numExitingNodes {
				timestamp := time.Now().UTC()
				initiatedAt = &timestamp
				updateInitiated = true
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
		require.Len(t, nodes, numExitingNodes)
	})
}

func TestGetExitingNodesLoopIncomplete(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			cache = db.OverlayCache()

			numTotalNodes          = 5
			numExitingNodesLoopInc = 2
		)

		for i := 0; i < numTotalNodes; i++ {
			newID := testrand.NodeID()
			// add nodes to cache
			err := cache.UpdateAddress(ctx, &pb.Node{Id: newID}, overlay.NodeSelectionConfig{})
			require.NoError(t, err)

			var (
				initiatedAt         *time.Time
				completedAt         *time.Time
				finishedAt          *time.Time
				updateInitiated     = false
				updateLoopCompleted = false
				updateFinished      = false
			)

			// set some nodes to have an exiting status
			if i < numExitingNodesLoopInc {
				timestamp := time.Now().UTC()
				initiatedAt = &timestamp
				updateInitiated = true
			} else {
				ts1 := time.Now().UTC()
				ts2 := time.Now().UTC()
				initiatedAt = &ts1
				updateInitiated = true
				completedAt = &ts2
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

		nodes, err := cache.GetExitingNodesLoopIncomplete(ctx)
		require.NoError(t, err)
		require.Len(t, nodes, numExitingNodesLoopInc)
	})
}
