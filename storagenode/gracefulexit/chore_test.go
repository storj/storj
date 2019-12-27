// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
	"storj.io/storj/uplink"
)

func TestChore(t *testing.T) {
	successThreshold := 4
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: successThreshold + 2,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite1 := planet.Satellites[0]
		uplinkPeer := planet.Uplinks[0]

		satellite1.GracefulExit.Chore.Loop.Pause()

		rs := &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: successThreshold,
			MaxThreshold:     successThreshold,
		}

		err := uplinkPeer.UploadWithConfig(ctx, satellite1, rs, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		exitingNode, err := findNodeToExit(ctx, planet, 1)
		require.NoError(t, err)

		nodePieceCounts, err := getNodePieceCounts(ctx, planet)
		require.NoError(t, err)

		exitSatellite(ctx, t, planet, exitingNode)

		newNodePieceCounts, err := getNodePieceCounts(ctx, planet)
		require.NoError(t, err)
		var newExitingNodeID storj.NodeID
		for k, v := range newNodePieceCounts {
			if v > nodePieceCounts[k] {
				newExitingNodeID = k
			}
		}
		require.NotNil(t, newExitingNodeID)
		require.NotEqual(t, exitingNode.ID(), newExitingNodeID)

		var newExitingNode *storagenode.Peer
		for _, node := range planet.StorageNodes {
			if node.ID() == newExitingNodeID {
				newExitingNode = node
			}
		}
		require.NotNil(t, newExitingNode)

		exitSatellite(ctx, t, planet, newExitingNode)
	})
}

func exitSatellite(ctx context.Context, t *testing.T, planet *testplanet.Planet, exitingNode *storagenode.Peer) {
	satellite1 := planet.Satellites[0]
	exitingNode.GracefulExit.Chore.Loop.Pause()

	startingDiskUsage, err := exitingNode.Storage2.BlobsCache.SpaceUsedBySatellite(ctx, satellite1.ID())
	require.NoError(t, err)
	require.NotZero(t, startingDiskUsage)

	exitStatus := overlay.ExitStatusRequest{
		NodeID:          exitingNode.ID(),
		ExitInitiatedAt: time.Now(),
	}

	_, err = satellite1.Overlay.DB.UpdateExitStatus(ctx, &exitStatus)
	require.NoError(t, err)

	err = exitingNode.DB.Satellites().InitiateGracefulExit(ctx, satellite1.ID(), time.Now(), startingDiskUsage)
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
	require.Equal(t, exitingNode.ID(), exitingNodes[0].NodeID)

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
			require.NotNil(t, progress.CompletionReceipt)
			require.NotNil(t, progress.FinishedAt)
			require.EqualValues(t, progress.StartingDiskUsage, progress.BytesDeleted)
		}
	}

	// make sure there are no more pieces on the node.
	namespaces, err := exitingNode.DB.Pieces().ListNamespaces(ctx)
	require.NoError(t, err)
	for _, ns := range namespaces {
		err = exitingNode.DB.Pieces().WalkNamespace(ctx, ns, func(blobInfo storage.BlobInfo) error {
			return errs.New("found a piece on the node. this shouldn't happen.")
		})
		require.NoError(t, err)
	}
}

// getNodePieceCounts tallies all the pieces per node.
func getNodePieceCounts(ctx context.Context, planet *testplanet.Planet) (_ map[storj.NodeID]int, err error) {
	nodePieceCounts := make(map[storj.NodeID]int)
	for _, n := range planet.StorageNodes {
		node := n
		namespaces, err := node.DB.Pieces().ListNamespaces(ctx)
		if err != nil {
			return nil, err
		}
		for _, ns := range namespaces {
			err = node.DB.Pieces().WalkNamespace(ctx, ns, func(blobInfo storage.BlobInfo) error {
				nodePieceCounts[node.ID()]++
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return nodePieceCounts, err
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
