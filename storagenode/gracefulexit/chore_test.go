// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storagenode"
	"storj.io/storj/uplink"
)

func TestChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 9,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite1 := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		satellite1.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     4,
			RepairThreshold:  6,
			SuccessThreshold: 8,
			MaxThreshold:     8,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite1, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitingNode, err := findNodeToExit(ctx, planet, 1)
		require.NoError(t, err)
		exitingNode.GracefulExit.Chore.Loop.Pause()

		exitStatus := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now(),
		}

		_, err = satellite1.Overlay.DB.UpdateExitStatus(ctx, &exitStatus)
		require.NoError(t, err)

		err = exitingNode.DB.Satellites().InitiateGracefulExit(ctx, satellite1.ID(), time.Now(), 10000)
		require.NoError(t, err)

		// check that the storage node is exiting
		exitProgress, err := exitingNode.DB.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)
		require.Len(t, exitProgress, 1)

		// initiate graceful exit on satellite side by running the SN chore.
		exitingNode.GracefulExit.Chore.Loop.TriggerWait()

		// run the satellite chore to build the transfer queue.
		satellite1.GracefulExit.Chore.Loop.TriggerWait()

		// check that the satellite knows the storage node is exiting.
		exitingNodes, err := satellite1.DB.OverlayCache().GetExitingNodes(ctx)
		require.NoError(t, err)
		require.Len(t, exitingNodes, 1)
		require.Equal(t, exitingNode.ID(), exitingNodes[0])

		queueItems, err := satellite1.DB.GracefulExit().GetIncomplete(ctx, exitStatus.NodeID, 10, 0)
		require.NoError(t, err)
		require.Len(t, queueItems, 1)

		// run the SN chore again to start processing transfers.
		exitingNode.GracefulExit.Chore.Loop.TriggerWait()

		// check that there are no more items to process
		queueItems, err = satellite1.DB.GracefulExit().GetIncomplete(ctx, exitStatus.NodeID, 10, 0)
		require.NoError(t, err)
		require.Len(t, queueItems, 0)

		exitProgress, err = exitingNode.DB.Satellites().ListGracefulExits(ctx)
		require.NoError(t, err)
		for _, progress := range exitProgress {
			if progress.SatelliteID == satellite1.ID() {
				require.NotNil(t, progress.FinishedAt)
			}
		}
	})
}

// findNodeToExit selects the node storing the most pieces as the node to graceful exit.
func findNodeToExit(ctx context.Context, planet *testplanet.Planet, objects int) (*storagenode.Peer, error) {
	satellite := planet.Satellites[0]
	keys, err := satellite.Metainfo.Database.List(ctx, nil, objects)
	if err != nil {
		return nil, err
	}

	pieceCountMap := make(map[storj.NodeID]int, len(planet.StorageNodes))
	for _, sn := range planet.StorageNodes {
		pieceCountMap[sn.ID()] = 0
	}

	for _, key := range keys {
		pointer, err := satellite.Metainfo.Service.Get(ctx, string(key))
		if err != nil {
			return nil, err
		}
		pieces := pointer.GetRemote().GetRemotePieces()
		for _, piece := range pieces {
			pieceCountMap[piece.NodeId]++
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

	for _, sn := range planet.StorageNodes {
		if sn.ID() == exitingNodeID {
			return sn, nil
		}
	}

	return nil, nil
}
