// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
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

		satellite.GracefulExit.Chore.Loop.Pause()

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
		require.NoError(t, err)

		err = uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path2", testrand.Bytes(5*memory.KiB))
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
		require.Len(t, incompleteTransfers, 2)
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
		require.Len(t, incompleteTransfers, 2)

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

		satellite.GracefulExit.Chore.Loop.Pause()

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path1", testrand.Bytes(5*memory.KiB))
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
		keys, err := satellite.Metainfo.Database.List(ctx, nil, -1)
		require.NoError(t, err)

		var oldPointer *pb.Pointer
		var path []byte
		for _, key := range keys {
			p, err := satellite.Metainfo.Service.Get(ctx, metabase.SegmentKey(key))
			require.NoError(t, err)

			if p.GetRemote() != nil {
				oldPointer = p
				path = key
				break
			}
		}

		// remove a piece from the pointer
		require.NotNil(t, oldPointer)
		oldPointerBytes, err := pb.Marshal(oldPointer)
		require.NoError(t, err)
		newPointer := &pb.Pointer{}
		err = pb.Unmarshal(oldPointerBytes, newPointer)
		require.NoError(t, err)

		remotePieces := newPointer.GetRemote().GetRemotePieces()
		var newPieces []*pb.RemotePiece = make([]*pb.RemotePiece, len(remotePieces)-1)
		idx := 0
		for _, p := range remotePieces {
			if p.NodeId != nodeToRemove.ID() {
				newPieces[idx] = p
				idx++
			}
		}
		newPointer.Remote.RemotePieces = newPieces
		newPointerBytes, err := pb.Marshal(newPointer)
		require.NoError(t, err)
		err = satellite.Metainfo.Database.CompareAndSwap(ctx, storage.Key(path), oldPointerBytes, newPointerBytes)
		require.NoError(t, err)

		satellite.GracefulExit.Chore.Loop.TriggerWait()

		incompleteTransfers, err := satellite.DB.GracefulExit().GetIncomplete(ctx, exitingNode.ID(), 20, 0)
		require.NoError(t, err)
		require.Len(t, incompleteTransfers, 1)
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
