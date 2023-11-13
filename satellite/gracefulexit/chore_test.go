// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
)

func TestChore(t *testing.T) {
	var maximumInactiveTimeFrame = time.Second * 1
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 8,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.GracefulExit.MaxInactiveTimeFrame = maximumInactiveTimeFrame
					// this test can be removed when we are using time-based GE everywhere
					config.GracefulExit.TimeBased = false
				},
				testplanet.ReconfigureRS(4, 6, 8, 8),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		exitingNode := planet.StorageNodes[1]

		project, err := uplinkPeer.GetProject(ctx, satellite)
		require.NoError(t, err)
		defer func() { require.NoError(t, project.Close()) }()

		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path2", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		info, err := project.BeginUpload(ctx, "testbucket", "test/path3", nil)
		require.NoError(t, err)

		upload, err := project.UploadPart(ctx, "testbucket", "test/path3", info.UploadID, 1)
		require.NoError(t, err)

		_, err = upload.Write(testrand.Bytes(5 * memory.KiB))
		require.NoError(t, err)
		require.NoError(t, upload.Commit())

		exitStatusRequest := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now(),
		}

		_, err = satellite.Overlay.DB.UpdateExitStatus(ctx, &exitStatusRequest)
		require.NoError(t, err)

		exitingNodes, err := satellite.Overlay.DB.GetExitingNodes(ctx)
		require.NoError(t, err)
		nodeIDs := make(storj.NodeIDList, 0, len(exitingNodes))
		for _, exitingNode := range exitingNodes {
			if exitingNode.ExitLoopCompletedAt == nil {
				nodeIDs = append(nodeIDs, exitingNode.NodeID)
			}
		}
		require.Len(t, nodeIDs, 1)

		// run the satellite ranged loop to build the transfer queue.
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 3)
		for _, incomplete := range incompleteTransfers {
			require.True(t, incomplete.DurabilityRatio > 0)
			require.NotNil(t, incomplete.RootPieceID)
		}

		// test the other nodes don't have anything to transfer
		for _, node := range planet.StorageNodes {
			if node.ID() == exitingNode.ID() {
				continue
			}
			incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, node.ID(), 20, 0)
			require.NoError(t, err)
			require.Len(t, incompleteTransfers, 0)
		}

		exitingNodes, err = satellite.Overlay.DB.GetExitingNodes(ctx)
		require.NoError(t, err)
		nodeIDs = make(storj.NodeIDList, 0, len(exitingNodes))
		for _, exitingNode := range exitingNodes {
			if exitingNode.ExitLoopCompletedAt == nil {
				nodeIDs = append(nodeIDs, exitingNode.NodeID)
			}
		}
		require.Len(t, nodeIDs, 0)

		err = satellite.DB.GracefulExit().IncrementProgress(ctx, exitingNode.ID(), 0, 0, 0)
		require.NoError(t, err)

		incompleteTransfers, err = satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 3)

		// node should fail graceful exit if it has been inactive for maximum inactive time frame since last activity
		time.Sleep(maximumInactiveTimeFrame + time.Second*1)
		// run the satellite ranged loop to build the transfer queue.
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		exitStatus, err := satellite.Overlay.DB.GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.False(t, exitStatus.ExitSuccess)
		require.NotNil(t, exitStatus.ExitFinishedAt)

		incompleteTransfers, err = satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 0)

	})
}

func TestChoreDurabilityRatio(t *testing.T) {
	const (
		maximumInactiveTimeFrame = time.Second * 1
		successThreshold         = 4
	)
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.GracefulExit.MaxInactiveTimeFrame = maximumInactiveTimeFrame
					// this test can be removed when we are using time-based GE everywhere
					config.GracefulExit.TimeBased = false
				},
				testplanet.ReconfigureRS(2, 3, successThreshold, 4),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		nodeToRemove := planet.StorageNodes[0]
		exitingNode := planet.StorageNodes[1]

		project, err := uplinkPeer.GetProject(ctx, satellite)
		require.NoError(t, err)
		defer func() { require.NoError(t, project.Close()) }()

		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		info, err := project.BeginUpload(ctx, "testbucket", "test/path2", nil)
		require.NoError(t, err)

		upload, err := project.UploadPart(ctx, "testbucket", "test/path2", info.UploadID, 1)
		require.NoError(t, err)

		_, err = upload.Write(testrand.Bytes(5 * memory.KiB))
		require.NoError(t, err)
		require.NoError(t, upload.Commit())

		exitStatusRequest := overlay.ExitStatusRequest{
			NodeID:          exitingNode.ID(),
			ExitInitiatedAt: time.Now(),
		}

		_, err = satellite.Overlay.DB.UpdateExitStatus(ctx, &exitStatusRequest)
		require.NoError(t, err)

		exitingNodes, err := satellite.Overlay.DB.GetExitingNodes(ctx)
		require.NoError(t, err)
		nodeIDs := make(storj.NodeIDList, 0, len(exitingNodes))
		for _, exitingNode := range exitingNodes {
			if exitingNode.ExitLoopCompletedAt == nil {
				nodeIDs = append(nodeIDs, exitingNode.NodeID)
			}
		}
		require.Len(t, nodeIDs, 1)

		// retrieve remote segment
		segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 2)

		for _, segment := range segments {
			remotePieces := segment.Pieces
			var newPieces metabase.Pieces = make(metabase.Pieces, len(remotePieces)-1)
			idx := 0
			for _, p := range remotePieces {
				if p.StorageNode != nodeToRemove.ID() {
					newPieces[idx] = p
					idx++
				}
			}
			err = satellite.Metabase.DB.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
				StreamID: segment.StreamID,
				Position: segment.Position,

				OldPieces:     segment.Pieces,
				NewPieces:     newPieces,
				NewRedundancy: segment.Redundancy,
			})
			require.NoError(t, err)
		}

		// run the satellite ranged loop to build the transfer queue.
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 2)
		for _, incomplete := range incompleteTransfers {
			require.Equal(t, float64(successThreshold-1)/float64(successThreshold), incomplete.DurabilityRatio)
			require.NotNil(t, incomplete.RootPieceID)
		}
	})
}
