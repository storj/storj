// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/overlay"
)

func TestChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 8,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite1 := planet.Satellites[0]
		exitingNode := planet.StorageNodes[0]

		satellite1.GracefulExit.Chore.Loop.Pause()
		exitingNode.GracefulExit.Chore.Loop.Pause()

		exitStatus := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now(),
		}

		_, err := satellite1.Overlay.DB.UpdateExitStatus(ctx, &exitStatus)
		require.NoError(t, err)

		err = exitingNode.DB.Satellites().InitiateGracefulExit(ctx, satellite1.ID(), time.Now(), 10000)
		require.NoError(t, err)

		exitProgress, err := exitingNode.DB.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)
		require.Len(t, exitProgress, 1)

		exitingNode.GracefulExit.Chore.Loop.TriggerWait()

		exitProgress, err = exitingNode.DB.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)

		for _, progress := range exitProgress {
			if progress.SatelliteID == satellite1.ID() {
				require.NotNil(t, progress.FinishedAt)
			}
		}
	})
}
