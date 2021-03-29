// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/uplink/private/etag"
	"storj.io/uplink/private/multipart"
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

		satellite.GracefulExit.Chore.Loop.Pause()

		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path2", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		info, err := multipart.NewMultipartUpload(ctx, project, "testbucket", "test/path3", nil)
		require.NoError(t, err)

		_, err = multipart.PutObjectPart(ctx, project, "testbucket", "test/path3", info.StreamID, 1,
			etag.NewHashReader(bytes.NewReader(testrand.Bytes(5*memory.KiB)), sha256.New()))
		require.NoError(t, err)

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

		satellite.GracefulExit.Chore.Loop.TriggerWait()

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

		satellite.GracefulExit.Chore.Loop.Pause()
		err = satellite.DB.GracefulExit().IncrementProgress(ctx, exitingNode.ID(), 0, 0, 0)
		require.NoError(t, err)

		incompleteTransfers, err = satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 3)

		// node should fail graceful exit if it has been inactive for maximum inactive time frame since last activity
		time.Sleep(maximumInactiveTimeFrame + time.Second*1)
		satellite.GracefulExit.Chore.Loop.TriggerWait()

		exitStatus, err := satellite.Overlay.DB.GetExitStatus(ctx, exitingNode.ID())
		require.NoError(t, err)
		require.False(t, exitStatus.ExitSuccess)
		require.NotNil(t, exitStatus.ExitFinishedAt)

		incompleteTransfers, err = satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 0)

	})
}

func TestDurabilityRatio(t *testing.T) {
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
		satellite.GracefulExit.Chore.Loop.Pause()

		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		info, err := multipart.NewMultipartUpload(ctx, project, "testbucket", "test/path2", nil)
		require.NoError(t, err)

		_, err = multipart.PutObjectPart(ctx, project, "testbucket", "test/path2", info.StreamID, 1,
			etag.NewHashReader(bytes.NewReader(testrand.Bytes(5*memory.KiB)), sha256.New()))
		require.NoError(t, err)

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
		segments, err := satellite.Metainfo.Metabase.TestingAllSegments(ctx)
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
			err = satellite.Metainfo.Metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
				StreamID: segment.StreamID,
				Position: segment.Position,

				OldPieces:     segment.Pieces,
				NewPieces:     newPieces,
				NewRedundancy: segment.Redundancy,
			})
			require.NoError(t, err)
		}

		satellite.GracefulExit.Chore.Loop.TriggerWait()

		incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 2)
		for _, incomplete := range incompleteTransfers {
			require.Equal(t, float64(successThreshold-1)/float64(successThreshold), incomplete.DurabilityRatio)
			require.NotNil(t, incomplete.RootPieceID)
		}
	})
}

func BenchmarkChore(b *testing.B) {
	satellitedbtest.Bench(b, func(b *testing.B, db satellite.DB) {
		gracefulexitdb := db.GracefulExit()
		ctx := context.Background()

		b.Run("BatchUpdateStats-100", func(b *testing.B) {
			batch(ctx, b, gracefulexitdb, 100)
		})
		if !testing.Short() {
			b.Run("BatchUpdateStats-250", func(b *testing.B) {
				batch(ctx, b, gracefulexitdb, 250)
			})
			b.Run("BatchUpdateStats-500", func(b *testing.B) {
				batch(ctx, b, gracefulexitdb, 500)
			})
			b.Run("BatchUpdateStats-1000", func(b *testing.B) {
				batch(ctx, b, gracefulexitdb, 1000)
			})
			b.Run("BatchUpdateStats-5000", func(b *testing.B) {
				batch(ctx, b, gracefulexitdb, 5000)
			})
		}
	})
}
func batch(ctx context.Context, b *testing.B, db gracefulexit.DB, size int) {
	for i := 0; i < b.N; i++ {
		var transferQueueItems []gracefulexit.TransferQueueItem
		for j := 0; j < size; j++ {
			item := gracefulexit.TransferQueueItem{
				NodeID:          testrand.NodeID(),
				Key:             testrand.Bytes(memory.B * 256),
				PieceNum:        0,
				DurabilityRatio: 1.0,
			}
			transferQueueItems = append(transferQueueItems, item)
		}
		err := db.Enqueue(ctx, transferQueueItems)
		require.NoError(b, err)
	}
}
