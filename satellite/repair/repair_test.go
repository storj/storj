// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repair_test

import (
	"context"
	"io"
	"math"
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
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode"
	"storj.io/storj/uplink"
)

// TestDataRepair does the following:
// - Uploads test data
// - Kills some nodes and disqualifies 1
// - Triggers data repair, which repairs the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
// - Shuts down several nodes, but keeping up a number equal to the minim
//	 threshold
// - Downloads the data from those left nodes and check that it's the same than
//   the uploaded one
func TestDataRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		var (
			testData         = testrand.Bytes(8 * memory.KiB)
			minThreshold     = 3
			successThreshold = 7
		)
		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     minThreshold,
			RepairThreshold:  5,
			SuccessThreshold: successThreshold,
			MaxThreshold:     9,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, satellite)

		// calculate how many storagenodes to kill
		redundancy := pointer.GetRemote().GetRedundancy()
		minReq := redundancy.GetMinReq()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		numPieces := len(remotePieces)
		// disqualify one storage node
		toDisqualify := 1
		toKill := numPieces - toDisqualify - int(minReq)
		require.True(t, toKill >= 1)
		maxNumRepairedPieces := int(
			math.Ceil(
				float64(successThreshold) * (1 + RepairMaxExcessRateOptimalThreshold),
			),
		)
		numStorageNodes := len(planet.StorageNodes)
		// Ensure that there are enough storage nodes to upload repaired segments
		require.Falsef(t,
			(numStorageNodes-toKill-toDisqualify) < maxNumRepairedPieces,
			"there is not enough available nodes for repairing: need= %d, have= %d",
			maxNumRepairedPieces, numStorageNodes-toKill-toDisqualify,
		)

		// kill nodes and track lost pieces
		nodesToKill := make(map[storj.NodeID]bool)
		nodesToDisqualify := make(map[storj.NodeID]bool)
		nodesToKeepAlive := make(map[storj.NodeID]bool)

		var numDisqualified int
		for i, piece := range remotePieces {
			if i >= toKill {
				if numDisqualified < toDisqualify {
					nodesToDisqualify[piece.NodeId] = true
					numDisqualified++
				}
				nodesToKeepAlive[piece.NodeId] = true
				continue
			}
			nodesToKill[piece.NodeId] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToDisqualify[node.ID()] {
				disqualifyNode(t, ctx, satellite, node.ID())
				continue
			}
			if nodesToKill[node.ID()] {
				stopNodeByID(t, ctx, planet, node.ID())
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// repaired segment should not contain any piece in the killed and DQ nodes
		metainfoService := satellite.Metainfo.Service
		pointer, err = metainfoService.Get(ctx, path)
		require.NoError(t, err)

		nodesToKillForMinThreshold := len(remotePieces) - minThreshold
		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			require.NotContains(t, nodesToKill, piece.NodeId, "there shouldn't be pieces in killed nodes")
			require.NotContains(t, nodesToDisqualify, piece.NodeId, "there shouldn't be pieces in DQ nodes")

			require.Nil(t, piece.Hash, "piece hashes should be set to nil")

			// Kill the original nodes which were kept alive to ensure that we can
			// download from the new nodes that the repaired pieces have been uploaded
			if _, ok := nodesToKeepAlive[piece.NodeId]; ok && nodesToKillForMinThreshold > 0 {
				stopNodeByID(t, ctx, planet, piece.NodeId)
				nodesToKillForMinThreshold--
			}
		}
		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)
	})
}

// TestCorruptDataRepair_Failed does the following:
// - Uploads test data
// - Kills all but the minimum number of nodes carrying the uploaded segment
// - On one of the remaining nodes, corrupt the piece data being stored by that node
// - Triggers data repair, which attempts to repair the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
// - Expects that the repair failed and the pointer was not updated
func TestCorruptDataRepair_Failed(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  5,
			SuccessThreshold: 7,
			MaxThreshold:     9,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, satellite)

		// calculate how many storagenodes to kill
		redundancy := pointer.GetRemote().GetRedundancy()
		minReq := redundancy.GetMinReq()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		numPieces := len(remotePieces)
		toKill := numPieces - int(minReq)
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		nodesToKill := make(map[storj.NodeID]bool)
		originalNodes := make(map[storj.NodeID]bool)

		var corruptedNode *storagenode.Peer
		var corruptedNodeID storj.NodeID
		var corruptedPieceID storj.PieceID

		for i, piece := range remotePieces {
			originalNodes[piece.NodeId] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				// choose a node and pieceID to corrupt so repair fails
				if corruptedNodeID.IsZero() || corruptedPieceID.IsZero() {
					corruptedNodeID = piece.NodeId
					corruptedPieceID = pointer.GetRemote().RootPieceId.Derive(corruptedNodeID, piece.PieceNum)
				}
				continue
			}
			nodesToKill[piece.NodeId] = true
		}
		require.NotNil(t, corruptedNodeID)
		require.NotNil(t, corruptedPieceID)

		for _, node := range planet.StorageNodes {
			if node.ID() == corruptedNodeID {
				corruptedNode = node
			}
			if nodesToKill[node.ID()] {
				stopNodeByID(t, ctx, planet, node.ID())
			}
		}
		require.NotNil(t, corruptedNode)

		overlay := planet.Satellites[0].Overlay.Service
		node, err := overlay.Get(ctx, corruptedNodeID)
		require.NoError(t, err)
		corruptedNodeReputation := node.Reputation

		corruptPieceData(ctx, t, planet, corruptedNode, corruptedPieceID)

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// repair should update audit status as fail
		node, err = overlay.Get(ctx, corruptedNodeID)
		require.NoError(t, err)
		require.Equal(t, corruptedNodeReputation.AuditCount+1, node.Reputation.AuditCount)
		require.True(t, corruptedNodeReputation.AuditReputationBeta < node.Reputation.AuditReputationBeta)
		require.True(t, corruptedNodeReputation.AuditReputationAlpha >= node.Reputation.AuditReputationAlpha)

		// repair should fail, so segment should contain all the original nodes
		metainfoService := satellite.Metainfo.Service
		pointer, err = metainfoService.Get(ctx, path)
		require.NoError(t, err)

		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			require.Contains(t, originalNodes, piece.NodeId, "there should be no new nodes in pointer")
		}
	})
}

// TestCorruptDataRepair does the following:
// - Uploads test data
// - Kills some nodes carrying the uploaded segment but keep it above minimum requirement
// - On one of the remaining nodes, corrupt the piece data being stored by that node
// - Triggers data repair, which attempts to repair the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
// - Expects that the repair succeed and the pointer should not contain the corrupted piece
func TestCorruptDataRepair_Succeed(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  5,
			SuccessThreshold: 7,
			MaxThreshold:     9,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, satellite)

		// calculate how many storagenodes to kill
		redundancy := pointer.GetRemote().GetRedundancy()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		numPieces := len(remotePieces)
		toKill := numPieces - int(redundancy.RepairThreshold)
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		nodesToKill := make(map[storj.NodeID]bool)
		originalNodes := make(map[storj.NodeID]bool)

		var corruptedNode *storagenode.Peer
		var corruptedNodeID storj.NodeID
		var corruptedPieceID storj.PieceID
		var corruptedPiece *pb.RemotePiece

		for i, piece := range remotePieces {
			originalNodes[piece.NodeId] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				// choose a node and pieceID to corrupt so repair fails
				if corruptedNodeID.IsZero() || corruptedPieceID.IsZero() {
					corruptedNodeID = piece.NodeId
					corruptedPieceID = pointer.GetRemote().RootPieceId.Derive(corruptedNodeID, piece.PieceNum)
					corruptedPiece = piece
				}
				continue
			}
			nodesToKill[piece.NodeId] = true
		}
		require.NotNil(t, corruptedNodeID)
		require.NotNil(t, corruptedPieceID)
		require.NotNil(t, corruptedPiece)

		for _, node := range planet.StorageNodes {
			if node.ID() == corruptedNodeID {
				corruptedNode = node
			}
			if nodesToKill[node.ID()] {
				stopNodeByID(t, ctx, planet, node.ID())
			}
		}
		require.NotNil(t, corruptedNode)

		corruptPieceData(ctx, t, planet, corruptedNode, corruptedPieceID)

		overlay := planet.Satellites[0].Overlay.Service
		node, err := overlay.Get(ctx, corruptedNodeID)
		require.NoError(t, err)
		corruptedNodeReputation := node.Reputation

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// repair should update audit status as fail
		node, err = overlay.Get(ctx, corruptedNodeID)
		require.NoError(t, err)
		require.Equal(t, corruptedNodeReputation.AuditCount+1, node.Reputation.AuditCount)
		require.True(t, corruptedNodeReputation.AuditReputationBeta < node.Reputation.AuditReputationBeta)
		require.True(t, corruptedNodeReputation.AuditReputationAlpha >= node.Reputation.AuditReputationAlpha)

		// get the new pointer
		metainfoService := satellite.Metainfo.Service
		pointer, err = metainfoService.Get(ctx, path)
		require.NoError(t, err)

		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			require.NotEqual(t, piece.PieceNum, corruptedPiece.PieceNum, "there should be no corrupted piece in pointer")
		}
	})
}

// TestRemoveDeletedSegmentFromQueue
// - Upload tests data to 7 nodes
// - Kill nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Delete segment from the satellite database
// - Run the repairer
// - Verify segment is no longer in the repair queue
func TestRemoveDeletedSegmentFromQueue(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Stop()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  5,
			SuccessThreshold: 7,
			MaxThreshold:     7,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, _ := getRemoteSegment(t, ctx, satellite)

		// kill nodes and track lost pieces
		nodesToDQ := make(map[storj.NodeID]bool)

		// Kill 3 nodes so that pointer has 4 left (less than repair threshold)
		toKill := 3

		remotePieces := pointer.GetRemote().GetRemotePieces()

		for i, piece := range remotePieces {
			if i >= toKill {
				continue
			}
			nodesToDQ[piece.NodeId] = true
		}

		for nodeID := range nodesToDQ {
			disqualifyNode(t, ctx, satellite, nodeID)
		}

		// trigger checker to add segment to repair queue
		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()

		// Delete segment from the satellite database
		err = uplinkPeer.Delete(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)

		// Verify that the segment is on the repair queue
		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 1)

		// Run the repairer
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 0)
	})
}

// TestRemoveIrreparableSegmentFromQueue
// - Upload tests data to 7 nodes
// - Kill nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Kill nodes so that online nodes < minimum threshold
// - Run the repairer
// - Verify segment is no longer in the repair queue and segment should be the same
func TestRemoveIrreparableSegmentFromQueue(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Stop()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  5,
			SuccessThreshold: 7,
			MaxThreshold:     7,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, _ := getRemoteSegment(t, ctx, satellite)

		// kill nodes and track lost pieces
		nodesToDQ := make(map[storj.NodeID]bool)

		// Kill 3 nodes so that pointer has 4 left (less than repair threshold)
		toKill := 3

		remotePieces := pointer.GetRemote().GetRemotePieces()

		for i, piece := range remotePieces {
			if i >= toKill {
				continue
			}
			nodesToDQ[piece.NodeId] = true
		}

		for nodeID := range nodesToDQ {
			disqualifyNode(t, ctx, satellite, nodeID)
		}

		// trigger checker to add segment to repair queue
		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()

		// Kill nodes so that online nodes < minimum threshold
		// This will make the segment irreparable
		for _, piece := range remotePieces {
			disqualifyNode(t, ctx, satellite, piece.NodeId)
		}

		// Verify that the segment is on the repair queue
		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 1)

		// Run the repairer
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 0)
	})
}

// TestRepairMultipleDisqualified does the following:
// - Uploads test data to 7 nodes
// - Disqualifies 3 nodes
// - Triggers data repair, which repairs the data from the remaining 4 nodes to additional 3 new nodes
// - Shuts down the 4 nodes from which the data was repaired
// - Now we have just the 3 new nodes to which the data was repaired
// - Downloads the data from these 3 nodes (succeeds because 3 nodes are enough for download)
func TestRepairMultipleDisqualified(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 12,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  5,
			SuccessThreshold: 7,
			MaxThreshold:     7,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from metainfo
		metainfo := satellite.Metainfo.Service
		listResponse, _, err := metainfo.List(ctx, "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = metainfo.Get(ctx, path)
			require.NoError(t, err)
			if pointer.GetType() == pb.Pointer_REMOTE {
				break
			}
		}

		// calculate how many storagenodes to disqualify
		numStorageNodes := len(planet.StorageNodes)
		redundancy := pointer.GetRemote().GetRedundancy()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		minReq := redundancy.GetMinReq()
		numPieces := len(remotePieces)
		toDisqualify := numPieces - (int(minReq + 1))
		// we should have enough storage nodes to repair on
		require.True(t, (numStorageNodes-toDisqualify) >= numPieces)

		// disqualify nodes and track lost pieces
		nodesToDisqualify := make(map[storj.NodeID]bool)
		nodesToKeepAlive := make(map[storj.NodeID]bool)

		for i, piece := range remotePieces {
			if i >= toDisqualify {
				nodesToKeepAlive[piece.NodeId] = true
				continue
			}
			nodesToDisqualify[piece.NodeId] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToDisqualify[node.ID()] {
				disqualifyNode(t, ctx, satellite, node.ID())
			}
		}

		err = satellite.Repair.Checker.RefreshReliabilityCache(ctx)
		require.NoError(t, err)

		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Limiter.Wait()

		// kill nodes kept alive to ensure repair worked
		for _, node := range planet.StorageNodes {
			if nodesToKeepAlive[node.ID()] {
				stopNodeByID(t, ctx, planet, node.ID())
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)

		// updated pointer should not contain any of the disqualified nodes
		pointer, err = metainfo.Get(ctx, path)
		require.NoError(t, err)

		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			require.False(t, nodesToDisqualify[piece.NodeId])
		}
	})
}

// TestDataRepairOverride_HigherLimit does the following:
// - Uploads test data
// - Kills nodes to fall to the Repair Override Value of the checker but stays above the original Repair Threshold
// - Triggers data repair, which attempts to repair the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
func TestDataRepairOverride_HigherLimit(t *testing.T) {
	const repairOverride = 6

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Checker.RepairOverride = repairOverride
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  4,
			SuccessThreshold: 9,
			MaxThreshold:     9,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, satellite)

		// calculate how many storagenodes to kill
		// kill one nodes less than repair threshold to ensure we dont hit it.
		remotePieces := pointer.GetRemote().GetRemotePieces()
		numPieces := len(remotePieces)
		toKill := numPieces - repairOverride
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		nodesToKill := make(map[storj.NodeID]bool)
		originalNodes := make(map[storj.NodeID]bool)

		for i, piece := range remotePieces {
			originalNodes[piece.NodeId] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				continue
			}
			nodesToKill[piece.NodeId] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				stopNodeByID(t, ctx, planet, node.ID())
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// repair should have been done, due to the override
		metainfoService := satellite.Metainfo.Service
		pointer, err = metainfoService.Get(ctx, path)
		require.NoError(t, err)

		// pointer should have the success count of pieces
		remotePieces = pointer.GetRemote().GetRemotePieces()
		require.Equal(t, int(pointer.Remote.Redundancy.SuccessThreshold), len(remotePieces))
	})
}

// TestDataRepairOverride_LowerLimit does the following:
// - Uploads test data
// - Kills nodes to fall to the Repair Threshold of the checker that should not trigger repair any longer
// - Starts Checker and Repairer and ensures this is the case.
// - Kills more nodes to fall to the Override Value to trigger repair
// - Triggers data repair, which attempts to repair the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
func TestDataRepairOverride_LowerLimit(t *testing.T) {
	const repairOverride = 4

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Checker.RepairOverride = repairOverride
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  6,
			SuccessThreshold: 9,
			MaxThreshold:     9,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, satellite)

		// calculate how many storagenodes to kill
		// to hit the repair threshold
		remotePieces := pointer.GetRemote().GetRemotePieces()
		repairThreshold := int(pointer.GetRemote().Redundancy.RepairThreshold)
		numPieces := len(remotePieces)
		toKill := numPieces - repairThreshold
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		nodesToKill := make(map[storj.NodeID]bool)
		originalNodes := make(map[storj.NodeID]bool)

		for i, piece := range remotePieces {
			originalNodes[piece.NodeId] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				continue
			}
			nodesToKill[piece.NodeId] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				stopNodeByID(t, ctx, planet, node.ID())
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// Increase offline count by the difference to trigger repair
		toKill += repairThreshold - repairOverride

		for i, piece := range remotePieces {
			originalNodes[piece.NodeId] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				continue
			}
			nodesToKill[piece.NodeId] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				err = planet.StopPeer(node)
				require.NoError(t, err)
				_, err = satellite.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// repair should have been done, due to the override
		metainfoService := satellite.Metainfo.Service
		pointer, err = metainfoService.Get(ctx, path)
		require.NoError(t, err)

		// pointer should have the success count of pieces
		remotePieces = pointer.GetRemote().GetRemotePieces()
		require.Equal(t, int(pointer.Remote.Redundancy.SuccessThreshold), len(remotePieces))
	})
}

// TestDataRepairUploadLimits does the following:
// - Uploads test data to nodes
// - Get one segment of that data to check in which nodes its pieces are stored
// - Kills as many nodes as needed which store such segment pieces
// - Triggers data repair
// - Verify that the number of pieces which repaired has uploaded don't overpass
//	 the established limit (success threshold + % of excess)
func TestDataRepairUploadLimit(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 13,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		const (
			repairThreshold  = 5
			successThreshold = 7
			maxThreshold     = 9
		)
		var (
			maxRepairUploadThreshold = int(
				math.Ceil(
					float64(successThreshold) * (1 + RepairMaxExcessRateOptimalThreshold),
				),
			)
			ul       = planet.Uplinks[0]
			testData = testrand.Bytes(8 * memory.KiB)
		)

		err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  repairThreshold,
			SuccessThreshold: successThreshold,
			MaxThreshold:     maxThreshold,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, satellite)
		originalPieces := pointer.GetRemote().GetRemotePieces()
		require.True(t, len(originalPieces) <= maxThreshold)

		{ // Check that there is enough nodes in the network which don't contain
			// pieces of the segment for being able to repair the lost pieces
			availableNumNodes := len(planet.StorageNodes) - len(originalPieces)
			neededNodesForRepair := maxRepairUploadThreshold - repairThreshold
			require.Truef(t,
				availableNumNodes >= neededNodesForRepair,
				"Not enough remaining nodes in the network for repairing the pieces: have= %d, need= %d",
				availableNumNodes, neededNodesForRepair,
			)
		}

		originalStorageNodes := make(map[storj.NodeID]struct{})
		for _, p := range originalPieces {
			originalStorageNodes[p.NodeId] = struct{}{}
		}

		killedNodes := make(map[storj.NodeID]struct{})
		{ // Register nodes of the network which don't have pieces for the segment
			// to be injured and ill nodes which have pieces of the segment in order
			// to injure it
			numNodesToKill := len(originalPieces) - repairThreshold
			for _, node := range planet.StorageNodes {
				if _, ok := originalStorageNodes[node.ID()]; !ok {
					continue
				}

				if len(killedNodes) < numNodesToKill {
					stopNodeByID(t, ctx, planet, node.ID())

					killedNodes[node.ID()] = struct{}{}
				}
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.Limiter.Wait()

		// Get the pointer after repair to check the nodes where the pieces are
		// stored
		pointer, err = satellite.Metainfo.Service.Get(ctx, path)
		require.NoError(t, err)

		// Check that repair has uploaded missed pieces to an expected number of
		// nodes
		afterRepairPieces := pointer.GetRemote().GetRemotePieces()
		require.Falsef(t,
			len(afterRepairPieces) > maxRepairUploadThreshold,
			"Repaired pieces cannot be over max repair upload threshold. maxRepairUploadThreshold= %d, have= %d",
			maxRepairUploadThreshold, len(afterRepairPieces),
		)
		require.Falsef(t,
			len(afterRepairPieces) < successThreshold,
			"Repaired pieces shouldn't be under success threshold. successThreshold= %d, have= %d",
			successThreshold, len(afterRepairPieces),
		)

		// Check that after repair, the segment doesn't have more pieces on the
		// killed nodes
		for _, p := range afterRepairPieces {
			require.NotContains(t, killedNodes, p.NodeId, "there shouldn't be pieces in killed nodes")
		}
	})
}

func isDisqualified(t *testing.T, ctx *testcontext.Context, satellite *testplanet.SatelliteSystem, nodeID storj.NodeID) bool {
	node, err := satellite.Overlay.Service.Get(ctx, nodeID)
	require.NoError(t, err)

	return node.Disqualified != nil
}

func disqualifyNode(t *testing.T, ctx *testcontext.Context, satellite *testplanet.SatelliteSystem, nodeID storj.NodeID) {
	_, err := satellite.DB.OverlayCache().UpdateStats(ctx, &overlay.UpdateRequest{
		NodeID:       nodeID,
		IsUp:         true,
		AuditSuccess: false,
		AuditLambda:  0,
		AuditWeight:  1,
		AuditDQ:      0.5,
		UptimeLambda: 1,
		UptimeWeight: 1,
		UptimeDQ:     0.5,
	})
	require.NoError(t, err)
	require.True(t, isDisqualified(t, ctx, satellite, nodeID))
}

// getRemoteSegment returns a remote pointer its path from satellite.
// nolint:golint
func getRemoteSegment(
	t *testing.T, ctx context.Context, satellite *testplanet.SatelliteSystem,
) (_ *pb.Pointer, path string) {
	t.Helper()

	// get a remote segment from metainfo
	metainfo := satellite.Metainfo.Service
	listResponse, _, err := metainfo.List(ctx, "", "", true, 0, 0)
	require.NoError(t, err)

	for _, v := range listResponse {
		path := v.GetPath()
		pointer, err := metainfo.Get(ctx, path)
		require.NoError(t, err)
		if pointer.GetType() == pb.Pointer_REMOTE {
			return pointer, path
		}
	}

	t.Fatal("satellite doesn't have any remote segment")
	return nil, ""
}

// nolint:golint
func stopNodeByID(t *testing.T, ctx context.Context, planet *testplanet.Planet, nodeID storj.NodeID) {
	t.Helper()

	for _, node := range planet.StorageNodes {
		if node.ID() == nodeID {
			err := planet.StopPeer(node)
			require.NoError(t, err)

			for _, satellite := range planet.Satellites {
				err = satellite.Overlay.Service.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
					NodeID: node.ID(),
					Address: &pb.NodeAddress{
						Address: node.Addr(),
					},
					IsUp: true,
					Version: &pb.NodeVersion{
						Version:    "v0.0.0",
						CommitHash: "",
						Timestamp:  time.Time{},
						Release:    false,
					},
				}, time.Now().UTC().Add(-4*time.Hour))
				require.NoError(t, err)
			}

			break
		}
	}
}

// corruptPieceData manipulates piece data on a storage node.
func corruptPieceData(ctx context.Context, t *testing.T, planet *testplanet.Planet, corruptedNode *storagenode.Peer, corruptedPieceID storj.PieceID) {
	t.Helper()

	blobRef := storage.BlobRef{
		Namespace: planet.Satellites[0].ID().Bytes(),
		Key:       corruptedPieceID.Bytes(),
	}

	// get currently stored piece data from storagenode
	reader, err := corruptedNode.Storage2.BlobsCache.Open(ctx, blobRef)
	require.NoError(t, err)
	pieceSize, err := reader.Size()
	require.NoError(t, err)
	require.True(t, pieceSize > 0)
	pieceData := make([]byte, pieceSize)
	n, err := io.ReadFull(reader, pieceData)
	require.NoError(t, err)
	require.EqualValues(t, n, pieceSize)

	// delete piece data
	err = corruptedNode.Storage2.BlobsCache.Delete(ctx, blobRef)
	require.NoError(t, err)

	// corrupt piece data (not PieceHeader) and write back to storagenode
	// this means repair downloading should fail during piece hash verification
	pieceData[pieceSize-1]++ // if we don't do this, this test should fail
	writer, err := corruptedNode.Storage2.BlobsCache.Create(ctx, blobRef, pieceSize)
	require.NoError(t, err)

	n, err = writer.Write(pieceData)
	require.NoError(t, err)
	require.EqualValues(t, n, pieceSize)

	err = writer.Commit(ctx)
	require.NoError(t, err)
}
