// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
)

func TestChore(t *testing.T) {
	const successThreshold = 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 2,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, successThreshold, successThreshold),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite1 := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		err := uplinkPeer.Upload(ctx, satellite1, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitingNode, err := findNodeToExit(ctx, planet)
		require.NoError(t, err)

		exitSatellite(ctx, t, planet, exitingNode)
	})
}

func exitSatellite(ctx context.Context, t *testing.T, planet *testplanet.Planet, exitingNode *testplanet.StorageNode) {
	satellite1 := planet.Satellites[0]
	exitingNode.GracefulExit.Chore.Loop.Pause()

	exitStatus := overlay.ExitStatusRequest{
		NodeID:          exitingNode.ID(),
		ExitInitiatedAt: time.Now(),
	}
	var timeMutex sync.Mutex
	var timeForward time.Duration
	satellite1.GracefulExit.Endpoint.SetNowFunc(func() time.Time {
		timeMutex.Lock()
		defer timeMutex.Unlock()
		return time.Now().Add(timeForward)
	})

	_, err := satellite1.Overlay.DB.UpdateExitStatus(ctx, &exitStatus)
	require.NoError(t, err)

	err = exitingNode.DB.Satellites().InitiateGracefulExit(ctx, satellite1.ID(), time.Now(), 0)
	require.NoError(t, err)

	// check that the storage node is exiting
	exitProgress, err := exitingNode.DB.Satellites().ListGracefulExits(ctx)
	require.NoError(t, err)
	require.Len(t, exitProgress, 1)

	// initiate graceful exit on satellite side by running the SN chore.
	exitingNode.GracefulExit.Chore.Loop.Pause()
	exitingNode.GracefulExit.Chore.Loop.TriggerWait()

	// jump ahead in time (the +2 is to account for things like daylight savings shifts that may
	// be happening in the next while, since we're not using AddDate here).
	timeMutex.Lock()
	timeForward += time.Duration(satellite1.Config.GracefulExit.GracefulExitDurationInDays*24+2) * time.Hour
	timeMutex.Unlock()

	// check that the satellite knows the storage node is exiting.
	// Note we cannot use GetExitingNodes, because there's a background worker that may finish the exit before
	// we check here.
	exitingNodeDossier, err := satellite1.DB.OverlayCache().Get(ctx, exitingNode.ID())
	require.NoError(t, err)
	require.NotNil(t, exitingNodeDossier.ExitStatus.ExitInitiatedAt)

	// run the SN chore again to start processing transfers.
	exitingNode.GracefulExit.Chore.Loop.TriggerWait()
	// wait for workers to finish
	err = exitingNode.GracefulExit.Chore.TestWaitForNoWorkers(ctx)
	require.NoError(t, err)

	exitProgress, err = exitingNode.DB.Satellites().ListGracefulExits(ctx)
	require.NoError(t, err)
	for _, progress := range exitProgress {
		if progress.SatelliteID == satellite1.ID() {
			require.NotNil(t, progress.CompletionReceipt)
			require.NotNil(t, progress.FinishedAt)
			require.EqualValues(t, 0, progress.BytesDeleted)
		}
	}
}

// findNodeToExit selects the node storing the most pieces as the node to graceful exit.
func findNodeToExit(ctx context.Context, planet *testplanet.Planet) (*testplanet.StorageNode, error) {
	satellite := planet.Satellites[0]

	objects, err := satellite.Metabase.DB.TestingAllSegments(ctx)
	if err != nil {
		return nil, err
	}

	pieceCountMap := make(map[storj.NodeID]int, len(planet.StorageNodes))
	for _, sn := range planet.StorageNodes {
		pieceCountMap[sn.ID()] = 0
	}

	for _, object := range objects {
		for _, piece := range object.Pieces {
			pieceCountMap[piece.StorageNode]++
		}
	}

	var exitingNodeID storj.NodeID
	maxCount := 0
	for k, v := range pieceCountMap {
		if exitingNodeID.IsZero() {
			exitingNodeID = k
			maxCount = v
			continue
		}
		if v > maxCount {
			exitingNodeID = k
			maxCount = v
		}
	}

	return planet.FindNode(exitingNodeID), nil
}
