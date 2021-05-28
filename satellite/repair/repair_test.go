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
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/storage"
)

// TestDataRepair does the following:
// - Uploads test data
// - Kills some nodes and disqualifies 1
// - Triggers data repair, which repairs the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
// - Shuts down several nodes, but keeping up a number equal to the minim
//	 threshold
// - Downloads the data from those left nodes and check that it's the same than the uploaded one.
func TestDataRepairInMemory(t *testing.T) {
	testDataRepair(t, true)
}
func TestDataRepairToDisk(t *testing.T) {
	testDataRepair(t, false)
}

func testDataRepair(t *testing.T, inMemoryRepair bool) {
	const (
		RepairMaxExcessRateOptimalThreshold = 0.05
		minThreshold                        = 3
		successThreshold                    = 7
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = inMemoryRepair
				},
				testplanet.ReconfigureRS(minThreshold, 5, successThreshold, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// calculate how many storagenodes to kill
		redundancy := segment.Redundancy
		minReq := redundancy.RequiredShares
		remotePieces := segment.Pieces
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
					nodesToDisqualify[piece.StorageNode] = true
					numDisqualified++
				}
				nodesToKeepAlive[piece.StorageNode] = true
				continue
			}
			nodesToKill[piece.StorageNode] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToDisqualify[node.ID()] {
				err := satellite.DB.OverlayCache().DisqualifyNode(ctx, node.ID())
				require.NoError(t, err)
				continue
			}
			if nodesToKill[node.ID()] {
				require.NoError(t, planet.StopNodeAndUpdate(ctx, node))
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// repaired segment should not contain any piece in the killed and DQ nodes
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		nodesToKillForMinThreshold := len(remotePieces) - minThreshold
		remotePieces = segmentAfter.Pieces
		for _, piece := range remotePieces {
			require.NotContains(t, nodesToKill, piece.StorageNode, "there shouldn't be pieces in killed nodes")
			require.NotContains(t, nodesToDisqualify, piece.StorageNode, "there shouldn't be pieces in DQ nodes")

			// Kill the original nodes which were kept alive to ensure that we can
			// download from the new nodes that the repaired pieces have been uploaded
			if _, ok := nodesToKeepAlive[piece.StorageNode]; ok && nodesToKillForMinThreshold > 0 {
				require.NoError(t, planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode)))
				nodesToKillForMinThreshold--
			}
		}
		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)
	})
}

// TestDataRepairPendingObject does the following:
// - Starts new multipart upload with one part of test data. Does not complete the multipart upload.
// - Kills some nodes and disqualifies 1
// - Triggers data repair, which repairs the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
// - Shuts down several nodes, but keeping up a number equal to the minim
//	 threshold
// - Completes the multipart upload.
// - Downloads the data from those left nodes and check that it's the same than the uploaded one.
func TestDataRepairPendingObjectInMemory(t *testing.T) {
	testDataRepairPendingObject(t, true)
}
func TestDataRepairPendingObjectToDisk(t *testing.T) {
	testDataRepairPendingObject(t, false)
}

func testDataRepairPendingObject(t *testing.T, inMemoryRepair bool) {
	const (
		RepairMaxExcessRateOptimalThreshold = 0.05
		minThreshold                        = 3
		successThreshold                    = 7
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = inMemoryRepair
				},
				testplanet.ReconfigureRS(minThreshold, 5, successThreshold, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		// first, start a new multipart upload and upload one part with some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		project, err := planet.Uplinks[0].OpenProject(ctx, planet.Satellites[0])
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		_, err = project.EnsureBucket(ctx, "testbucket")
		require.NoError(t, err)

		// upload pending object
		info, err := project.BeginUpload(ctx, "testbucket", "test/path", nil)
		require.NoError(t, err)
		upload, err := project.UploadPart(ctx, "testbucket", "test/path", info.UploadID, 7)
		require.NoError(t, err)
		_, err = upload.Write(testData)
		require.NoError(t, err)
		require.NoError(t, upload.Commit())

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// calculate how many storagenodes to kill
		redundancy := segment.Redundancy
		minReq := redundancy.RequiredShares
		remotePieces := segment.Pieces
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
					nodesToDisqualify[piece.StorageNode] = true
					numDisqualified++
				}
				nodesToKeepAlive[piece.StorageNode] = true
				continue
			}
			nodesToKill[piece.StorageNode] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToDisqualify[node.ID()] {
				err := satellite.DB.OverlayCache().DisqualifyNode(ctx, node.ID())
				require.NoError(t, err)
				continue
			}
			if nodesToKill[node.ID()] {
				require.NoError(t, planet.StopNodeAndUpdate(ctx, node))
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// repaired segment should not contain any piece in the killed and DQ nodes
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		nodesToKillForMinThreshold := len(remotePieces) - minThreshold
		remotePieces = segmentAfter.Pieces
		for _, piece := range remotePieces {
			require.NotContains(t, nodesToKill, piece.StorageNode, "there shouldn't be pieces in killed nodes")
			require.NotContains(t, nodesToDisqualify, piece.StorageNode, "there shouldn't be pieces in DQ nodes")

			// Kill the original nodes which were kept alive to ensure that we can
			// download from the new nodes that the repaired pieces have been uploaded
			if _, ok := nodesToKeepAlive[piece.StorageNode]; ok && nodesToKillForMinThreshold > 0 {
				require.NoError(t, planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode)))
				nodesToKillForMinThreshold--
			}
		}

		// complete the pending multipart upload
		_, err = project.CommitUpload(ctx, "testbucket", "test/path", info.UploadID, nil)
		require.NoError(t, err)

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
// - Expects that the repair failed and the pointer was not updated.
func TestCorruptDataRepairInMemory_Failed(t *testing.T) {
	testCorruptDataRepairFailed(t, true)
}
func TestCorruptDataRepairToDisk_Failed(t *testing.T) {
	testCorruptDataRepairFailed(t, false)
}

func testCorruptDataRepairFailed(t *testing.T, inMemoryRepair bool) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = inMemoryRepair
				},
				testplanet.ReconfigureRS(3, 5, 7, 9),
			),
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
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// calculate how many storagenodes to kill
		redundancy := segment.Redundancy
		minReq := redundancy.RequiredShares
		remotePieces := segment.Pieces
		numPieces := len(remotePieces)
		toKill := numPieces - int(minReq)
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		originalNodes := make(map[storj.NodeID]bool)

		var corruptedNodeID storj.NodeID
		var corruptedPieceID storj.PieceID

		for i, piece := range remotePieces {
			originalNodes[piece.StorageNode] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				// choose a node and pieceID to corrupt so repair fails
				if corruptedNodeID.IsZero() || corruptedPieceID.IsZero() {
					corruptedNodeID = piece.StorageNode
					corruptedPieceID = segment.RootPieceID.Derive(corruptedNodeID, int32(piece.Number))
				}
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.NotNil(t, corruptedNodeID)
		require.NotNil(t, corruptedPieceID)

		corruptedNode := planet.FindNode(corruptedNodeID)
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
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// repair should update audit status as fail
		node, err = overlay.Get(ctx, corruptedNodeID)
		require.NoError(t, err)
		require.Equal(t, corruptedNodeReputation.AuditCount+1, node.Reputation.AuditCount)
		require.True(t, corruptedNodeReputation.AuditReputationBeta < node.Reputation.AuditReputationBeta)
		require.True(t, corruptedNodeReputation.AuditReputationAlpha >= node.Reputation.AuditReputationAlpha)

		// repair should fail, so segment should contain all the original nodes
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		remotePieces = segmentAfter.Pieces
		for _, piece := range remotePieces {
			require.Contains(t, originalNodes, piece.StorageNode, "there should be no new nodes in pointer")
		}
	})
}

// TestCorruptDataRepair does the following:
// - Uploads test data
// - Kills some nodes carrying the uploaded segment but keep it above minimum requirement
// - On one of the remaining nodes, corrupt the piece data being stored by that node
// - Triggers data repair, which attempts to repair the data from the remaining nodes to
//	 the numbers of nodes determined by the upload repair max threshold
// - Expects that the repair succeed and the pointer should not contain the corrupted piece.
func TestCorruptDataRepairInMemory_Succeed(t *testing.T) {
	testCorruptDataRepairSucceed(t, true)
}
func TestCorruptDataRepairToDisk_Succeed(t *testing.T) {
	testCorruptDataRepairSucceed(t, false)
}

func testCorruptDataRepairSucceed(t *testing.T, inMemoryRepair bool) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = inMemoryRepair
				},
				testplanet.ReconfigureRS(3, 5, 7, 9),
			),
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
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// calculate how many storagenodes to kill
		redundancy := segment.Redundancy
		remotePieces := segment.Pieces
		numPieces := len(remotePieces)
		toKill := numPieces - int(redundancy.RepairShares)
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		originalNodes := make(map[storj.NodeID]bool)

		var corruptedNodeID storj.NodeID
		var corruptedPieceID storj.PieceID
		var corruptedPiece metabase.Piece

		for i, piece := range remotePieces {
			originalNodes[piece.StorageNode] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				// choose a node and pieceID to corrupt so repair fails
				if corruptedNodeID.IsZero() || corruptedPieceID.IsZero() {
					corruptedNodeID = piece.StorageNode
					corruptedPieceID = segment.RootPieceID.Derive(corruptedNodeID, int32(piece.Number))
					corruptedPiece = piece
				}
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.NotNil(t, corruptedNodeID)
		require.NotNil(t, corruptedPieceID)
		require.NotNil(t, corruptedPiece)

		corruptedNode := planet.FindNode(corruptedNodeID)
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
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// repair should update audit status as fail
		node, err = overlay.Get(ctx, corruptedNodeID)
		require.NoError(t, err)
		require.Equal(t, corruptedNodeReputation.AuditCount+1, node.Reputation.AuditCount)
		require.True(t, corruptedNodeReputation.AuditReputationBeta < node.Reputation.AuditReputationBeta)
		require.True(t, corruptedNodeReputation.AuditReputationAlpha >= node.Reputation.AuditReputationAlpha)

		// get the new segment
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		remotePieces = segmentAfter.Pieces
		for _, piece := range remotePieces {
			require.NotEqual(t, piece.Number, corruptedPiece.Number, "there should be no corrupted piece in pointer")
		}
	})
}

// TestRemoveExpiredSegmentFromQueue
// - Upload tests data to 7 nodes
// - Kill nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Modify segment to be expired
// - Run the repairer
// - Verify segment is still in the repair queue. We don't want the data repairer to have any special treatment for expired segment.
func TestRepairExpiredSegment(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 5, 7, 7),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Stop()
		satellite.Audit.Chore.Loop.Pause()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.UploadWithExpiration(ctx, satellite, "testbucket", "test/path", testData, time.Now().Add(1*time.Hour))
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// kill nodes and track lost pieces
		nodesToDQ := make(map[storj.NodeID]bool)

		// Kill 3 nodes so that pointer has 4 left (less than repair threshold)
		toKill := 3

		remotePieces := segment.Pieces

		for i, piece := range remotePieces {
			if i >= toKill {
				continue
			}
			nodesToDQ[piece.StorageNode] = true
		}

		for nodeID := range nodesToDQ {
			err := satellite.DB.OverlayCache().DisqualifyNode(ctx, nodeID)
			require.NoError(t, err)

		}

		// trigger checker to add segment to repair queue
		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()

		// get encrypted path of segment with audit service
		satellite.Audit.Chore.Loop.TriggerWait()
		queue := satellite.Audit.Queues.Fetch()
		require.EqualValues(t, queue.Size(), 1)

		// Verify that the segment is on the repair queue
		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		satellite.Repair.Repairer.SetNow(func() time.Time {
			return time.Now().Add(2 * time.Hour)
		})

		// Run the repairer
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the segment is still in the queue
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})
}

// TestRemoveDeletedSegmentFromQueue
// - Upload tests data to 7 nodes
// - Kill nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Delete segment from the satellite database
// - Run the repairer
// - Verify segment is no longer in the repair queue.
func TestRemoveDeletedSegmentFromQueue(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 5, 7, 7),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Stop()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// kill nodes and track lost pieces
		nodesToDQ := make(map[storj.NodeID]bool)

		// Kill 3 nodes so that pointer has 4 left (less than repair threshold)
		toKill := 3

		remotePieces := segment.Pieces

		for i, piece := range remotePieces {
			if i >= toKill {
				continue
			}
			nodesToDQ[piece.StorageNode] = true
		}

		for nodeID := range nodesToDQ {
			err := satellite.DB.OverlayCache().DisqualifyNode(ctx, nodeID)
			require.NoError(t, err)

		}

		// trigger checker to add segment to repair queue
		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()

		// Delete segment from the satellite database
		err = uplinkPeer.DeleteObject(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)

		// Verify that the segment is on the repair queue
		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 1)

		// Run the repairer
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 0)
	})
}

// TestIrreparableSegmentAccordingToOverlay
// - Upload tests data to 7 nodes
// - Disqualify nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Disqualify nodes so that online nodes < minimum threshold
// - Run the repairer
// - Verify segment is no longer in the repair queue and segment should be the same
// - Verify segment is now in the irreparable db instead.
func TestIrreparableSegmentAccordingToOverlay(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 5, 7, 7),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Stop()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Checker.IrreparableLoop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, segmentKey := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// dq 3 nodes so that pointer has 4 left (less than repair threshold)
		toDQ := 3
		remotePieces := segment.Pieces

		for i := 0; i < toDQ; i++ {
			err := satellite.DB.OverlayCache().DisqualifyNode(ctx, remotePieces[i].StorageNode)
			require.NoError(t, err)
		}

		// trigger checker to add segment to repair queue
		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()

		// Disqualify nodes so that online nodes < minimum threshold
		// This will make the segment irreparable
		for _, piece := range remotePieces {
			err := satellite.DB.OverlayCache().DisqualifyNode(ctx, piece.StorageNode)
			require.NoError(t, err)

		}

		// Verify that the segment is on the repair queue
		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 1)

		// Verify that the segment is not in the irreparable db
		irreparableSegment, err := satellite.DB.Irreparable().Get(ctx, segmentKey)
		require.Error(t, err)
		require.Nil(t, irreparableSegment)

		// Run the repairer
		beforeRepair := time.Now().Truncate(time.Second)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()
		afterRepair := time.Now().Truncate(time.Second)

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 0)

		// Verify that the segment _is_ in the irreparable db
		irreparableSegment, err = satellite.DB.Irreparable().Get(ctx, segmentKey)
		require.NoError(t, err)
		require.Equal(t, segmentKey, metabase.SegmentKey(irreparableSegment.Path))
		lastAttemptTime := time.Unix(irreparableSegment.LastRepairAttempt, 0)
		require.Falsef(t, lastAttemptTime.Before(beforeRepair), "%s is before %s", lastAttemptTime, beforeRepair)
		require.Falsef(t, lastAttemptTime.After(afterRepair), "%s is after %s", lastAttemptTime, afterRepair)
	})
}

func updateNodeCheckIn(ctx context.Context, overlayDB overlay.DB, node *testplanet.StorageNode, isUp bool, timestamp time.Time) error {
	local := node.Contact.Service.Local()
	checkInInfo := overlay.NodeCheckInInfo{
		NodeID: node.ID(),
		Address: &pb.NodeAddress{
			Address: local.Address,
		},
		LastIPPort: local.Address,
		IsUp:       isUp,
		Operator:   &local.Operator,
		Capacity:   &local.Capacity,
		Version:    &local.Version,
	}
	return overlayDB.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-24*time.Hour), overlay.NodeSelectionConfig{})
}

// TestIrreparableSegmentNodesOffline
// - Upload tests data to 7 nodes
// - Disqualify nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Kill (as opposed to disqualifying) nodes so that online nodes < minimum threshold
// - Run the repairer
// - Verify segment is no longer in the repair queue and segment should be the same
// - Verify segment is now in the irreparable db instead.
func TestIrreparableSegmentNodesOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 5, 7, 7),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Stop()

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Checker.IrreparableLoop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, segmentKey := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")

		// kill 3 nodes and mark them as offline so that pointer has 4 left from overlay
		// perspective (less than repair threshold)
		toMarkOffline := 3
		remotePieces := segment.Pieces

		for _, piece := range remotePieces[:toMarkOffline] {
			node := planet.FindNode(piece.StorageNode)

			err := planet.StopNodeAndUpdate(ctx, node)
			require.NoError(t, err)

			err = updateNodeCheckIn(ctx, satellite.DB.OverlayCache(), node, false, time.Now().Add(-24*time.Hour))
			require.NoError(t, err)
		}

		// trigger checker to add segment to repair queue
		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()

		// Verify that the segment is on the repair queue
		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 1)

		// Kill 2 extra nodes so that the number of available pieces is less than the minimum
		for _, piece := range remotePieces[toMarkOffline : toMarkOffline+2] {
			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}

		// Mark nodes as online again so that online nodes > minimum threshold
		// This will make the repair worker attempt to download the pieces
		for _, piece := range remotePieces[:toMarkOffline] {
			node := planet.FindNode(piece.StorageNode)
			err := updateNodeCheckIn(ctx, satellite.DB.OverlayCache(), node, true, time.Now())
			require.NoError(t, err)
		}

		// Verify that the segment is not in the irreparable db
		irreparableSegment, err := satellite.DB.Irreparable().Get(ctx, segmentKey)
		require.Error(t, err)
		require.Nil(t, irreparableSegment)

		// Run the repairer
		beforeRepair := time.Now().Truncate(time.Second)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()
		afterRepair := time.Now().Truncate(time.Second)

		// Verify that the segment was removed from the repair queue
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)

		// Verify that the segment _is_ in the irreparable db
		irreparableSegment, err = satellite.DB.Irreparable().Get(ctx, segmentKey)
		require.NoError(t, err)
		require.Equal(t, segmentKey, metabase.SegmentKey(irreparableSegment.Path))
		lastAttemptTime := time.Unix(irreparableSegment.LastRepairAttempt, 0)
		require.Falsef(t, lastAttemptTime.Before(beforeRepair), "%s is before %s", lastAttemptTime, beforeRepair)
		require.Falsef(t, lastAttemptTime.After(afterRepair), "%s is after %s", lastAttemptTime, afterRepair)
	})
}

// TestRepairMultipleDisqualifiedAndSuspended does the following:
// - Uploads test data to 7 nodes
// - Disqualifies 2 nodes and suspends 1 node
// - Triggers data repair, which repairs the data from the remaining 4 nodes to additional 3 new nodes
// - Shuts down the 4 nodes from which the data was repaired
// - Now we have just the 3 new nodes to which the data was repaired
// - Downloads the data from these 3 nodes (succeeds because 3 nodes are enough for download)
// - Expect newly repaired pointer does not contain the disqualified or suspended nodes.
func TestRepairMultipleDisqualifiedAndSuspendedInMemory(t *testing.T) {
	testRepairMultipleDisqualifiedAndSuspended(t, true)
}
func TestRepairMultipleDisqualifiedAndSuspendedToDisk(t *testing.T) {
	testRepairMultipleDisqualifiedAndSuspended(t, false)
}

func testRepairMultipleDisqualifiedAndSuspended(t *testing.T, inMemoryRepair bool) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 12,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = inMemoryRepair
				},
				testplanet.ReconfigureRS(3, 5, 7, 7),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from metainfo
		segments, err := satellite.Metainfo.Metabase.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.False(t, segments[0].Inline())

		// calculate how many storagenodes to disqualify
		numStorageNodes := len(planet.StorageNodes)
		remotePieces := segments[0].Pieces
		numPieces := len(remotePieces)
		// sanity check
		require.EqualValues(t, numPieces, 7)
		toDisqualify := 2
		toSuspend := 1
		// we should have enough storage nodes to repair on
		require.True(t, (numStorageNodes-toDisqualify-toSuspend) >= numPieces)

		// disqualify nodes and track lost pieces
		nodesToDisqualify := make(map[storj.NodeID]bool)
		nodesToSuspend := make(map[storj.NodeID]bool)
		nodesToKeepAlive := make(map[storj.NodeID]bool)

		// disqualify and suspend nodes
		for i := 0; i < toDisqualify; i++ {
			nodesToDisqualify[remotePieces[i].StorageNode] = true
			err := satellite.DB.OverlayCache().DisqualifyNode(ctx, remotePieces[i].StorageNode)
			require.NoError(t, err)
		}
		for i := toDisqualify; i < toDisqualify+toSuspend; i++ {
			nodesToSuspend[remotePieces[i].StorageNode] = true
			err := satellite.DB.OverlayCache().SuspendNodeUnknownAudit(ctx, remotePieces[i].StorageNode, time.Now())
			require.NoError(t, err)
		}
		for i := toDisqualify + toSuspend; i < len(remotePieces); i++ {
			nodesToKeepAlive[remotePieces[i].StorageNode] = true
		}

		err = satellite.Repair.Checker.RefreshReliabilityCache(ctx)
		require.NoError(t, err)

		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// kill nodes kept alive to ensure repair worked
		for _, node := range planet.StorageNodes {
			if nodesToKeepAlive[node.ID()] {
				err := planet.StopNodeAndUpdate(ctx, node)
				require.NoError(t, err)
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)

		segments, err = satellite.Metainfo.Metabase.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)

		remotePieces = segments[0].Pieces
		for _, piece := range remotePieces {
			require.False(t, nodesToDisqualify[piece.StorageNode])
			require.False(t, nodesToSuspend[piece.StorageNode])
		}
	})
}

// TestDataRepairOverride_HigherLimit does the following:
//   - Uploads test data
//   - Kills nodes to fall to the Repair Override Value of the checker but stays above the original Repair Threshold
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//	   the numbers of nodes determined by the upload repair max threshold
func TestDataRepairOverride_HigherLimitInMemory(t *testing.T) {
	testDataRepairOverrideHigherLimit(t, true)
}
func TestDataRepairOverride_HigherLimitToDisk(t *testing.T) {
	testDataRepairOverrideHigherLimit(t, false)
}

func testDataRepairOverrideHigherLimit(t *testing.T, inMemoryRepair bool) {
	const repairOverride = 6

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = inMemoryRepair
					config.Checker.RepairOverrides = checker.RepairOverrides{
						List: []checker.RepairOverride{
							{Min: 3, Success: 9, Total: 9, Override: repairOverride},
						},
					}
				},
				testplanet.ReconfigureRS(3, 4, 9, 9),
			),
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
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")

		// calculate how many storagenodes to kill
		// kill one nodes less than repair threshold to ensure we dont hit it.
		remotePieces := segment.Pieces
		numPieces := len(remotePieces)
		toKill := numPieces - repairOverride
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		nodesToKill := make(map[storj.NodeID]bool)
		originalNodes := make(map[storj.NodeID]bool)

		for i, piece := range remotePieces {
			originalNodes[piece.StorageNode] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				continue
			}
			nodesToKill[piece.StorageNode] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				err := planet.StopNodeAndUpdate(ctx, node)
				require.NoError(t, err)
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// repair should have been done, due to the override
		segment, _ = getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")

		// pointer should have the success count of pieces
		remotePieces = segment.Pieces
		require.Equal(t, int(segment.Redundancy.OptimalShares), len(remotePieces))
	})
}

// TestDataRepairOverride_LowerLimit does the following:
//   - Uploads test data
//   - Kills nodes to fall to the Repair Threshold of the checker that should not trigger repair any longer
//   - Starts Checker and Repairer and ensures this is the case.
//   - Kills more nodes to fall to the Override Value to trigger repair
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//	   the numbers of nodes determined by the upload repair max threshold
func TestDataRepairOverride_LowerLimitInMemory(t *testing.T) {
	testDataRepairOverrideLowerLimit(t, true)
}
func TestDataRepairOverride_LowerLimitToDisk(t *testing.T) {
	testDataRepairOverrideLowerLimit(t, false)
}

func testDataRepairOverrideLowerLimit(t *testing.T, inMemoryRepair bool) {
	const repairOverride = 4

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = inMemoryRepair
					config.Checker.RepairOverrides = checker.RepairOverrides{
						List: []checker.RepairOverride{
							{Min: 3, Success: 9, Total: 9, Override: repairOverride},
						},
					}
				},
				testplanet.ReconfigureRS(3, 6, 9, 9),
			),
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
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")

		// calculate how many storagenodes to kill
		// to hit the repair threshold
		remotePieces := segment.Pieces
		repairThreshold := int(segment.Redundancy.RepairShares)
		numPieces := len(remotePieces)
		toKill := numPieces - repairThreshold
		require.True(t, toKill >= 1)

		// kill nodes and track lost pieces
		nodesToKill := make(map[storj.NodeID]bool)
		originalNodes := make(map[storj.NodeID]bool)

		for i, piece := range remotePieces {
			originalNodes[piece.StorageNode] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				continue
			}
			nodesToKill[piece.StorageNode] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				err := planet.StopNodeAndUpdate(ctx, node)
				require.NoError(t, err)
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Increase offline count by the difference to trigger repair
		toKill += repairThreshold - repairOverride

		for i, piece := range remotePieces {
			originalNodes[piece.StorageNode] = true
			if i >= toKill {
				// this means the node will be kept alive for repair
				continue
			}
			nodesToKill[piece.StorageNode] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				err = planet.StopNodeAndUpdate(ctx, node)
				require.NoError(t, err)
			}
		}

		satellite.Repair.Checker.Loop.Restart()
		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// repair should have been done, due to the override
		segment, _ = getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")

		// pointer should have the success count of pieces
		remotePieces = segment.Pieces
		require.Equal(t, int(segment.Redundancy.OptimalShares), len(remotePieces))
	})
}

// TestDataRepairUploadLimits does the following:
//   - Uploads test data to nodes
//   - Get one segment of that data to check in which nodes its pieces are stored
//   - Kills as many nodes as needed which store such segment pieces
//   - Triggers data repair
//   - Verify that the number of pieces which repaired has uploaded don't overpass
//     the established limit (success threshold + % of excess)
func TestDataRepairUploadLimitInMemory(t *testing.T) {
	testDataRepairUploadLimit(t, true)
}
func TestDataRepairUploadLimitToDisk(t *testing.T) {
	testDataRepairUploadLimit(t, false)
}

func testDataRepairUploadLimit(t *testing.T, inMemoryRepair bool) {
	const (
		RepairMaxExcessRateOptimalThreshold = 0.05
		repairThreshold                     = 5
		successThreshold                    = 7
		maxThreshold                        = 9
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 13,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = inMemoryRepair
				},
				testplanet.ReconfigureRS(3, repairThreshold, successThreshold, maxThreshold),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		var (
			maxRepairUploadThreshold = int(
				math.Ceil(
					float64(successThreshold) * (1 + RepairMaxExcessRateOptimalThreshold),
				),
			)
			ul       = planet.Uplinks[0]
			testData = testrand.Bytes(8 * memory.KiB)
		)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, ul.Projects[0].ID, "testbucket")

		originalPieces := segment.Pieces
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
			originalStorageNodes[p.StorageNode] = struct{}{}
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
					err := planet.StopNodeAndUpdate(ctx, node)
					require.NoError(t, err)

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
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Get the pointer after repair to check the nodes where the pieces are
		// stored
		segment, _ = getRemoteSegment(ctx, t, satellite, ul.Projects[0].ID, "testbucket")

		// Check that repair has uploaded missed pieces to an expected number of
		// nodes
		afterRepairPieces := segment.Pieces
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
			require.NotContains(t, killedNodes, p.StorageNode, "there shouldn't be pieces in killed nodes")
		}
	})
}

// TestRepairGracefullyExited does the following:
// - Uploads test data to 7 nodes
// - Set 3 nodes as gracefully exited
// - Triggers data repair, which repairs the data from the remaining 4 nodes to additional 3 new nodes
// - Shuts down the 4 nodes from which the data was repaired
// - Now we have just the 3 new nodes to which the data was repaired
// - Downloads the data from these 3 nodes (succeeds because 3 nodes are enough for download)
// - Expect newly repaired pointer does not contain the gracefully exited nodes.
func TestRepairGracefullyExitedInMemory(t *testing.T) {
	testRepairGracefullyExited(t, true)
}
func TestRepairGracefullyExitedToDisk(t *testing.T) {
	testRepairGracefullyExited(t, false)
}

func testRepairGracefullyExited(t *testing.T, inMemoryRepair bool) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 12,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = inMemoryRepair
				},
				testplanet.ReconfigureRS(3, 5, 7, 7),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.Repair.Checker.Loop.Pause()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		numStorageNodes := len(planet.StorageNodes)
		remotePieces := segment.Pieces
		numPieces := len(remotePieces)
		// sanity check
		require.EqualValues(t, numPieces, 7)
		toExit := 3
		// we should have enough storage nodes to repair on
		require.True(t, (numStorageNodes-toExit) >= numPieces)

		// gracefully exit nodes and track lost pieces
		nodesToExit := make(map[storj.NodeID]bool)
		nodesToKeepAlive := make(map[storj.NodeID]bool)

		// exit nodes
		for i := 0; i < toExit; i++ {
			nodesToExit[remotePieces[i].StorageNode] = true
			req := &overlay.ExitStatusRequest{
				NodeID:              remotePieces[i].StorageNode,
				ExitInitiatedAt:     time.Now(),
				ExitLoopCompletedAt: time.Now(),
				ExitFinishedAt:      time.Now(),
			}
			_, err := satellite.DB.OverlayCache().UpdateExitStatus(ctx, req)
			require.NoError(t, err)
		}
		for i := toExit; i < len(remotePieces); i++ {
			nodesToKeepAlive[remotePieces[i].StorageNode] = true
		}

		err = satellite.Repair.Checker.RefreshReliabilityCache(ctx)
		require.NoError(t, err)

		satellite.Repair.Checker.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// kill nodes kept alive to ensure repair worked
		for _, node := range planet.StorageNodes {
			if nodesToKeepAlive[node.ID()] {
				require.NoError(t, planet.StopNodeAndUpdate(ctx, node))
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)

		// updated pointer should not contain any of the gracefully exited nodes
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		remotePieces = segmentAfter.Pieces
		for _, piece := range remotePieces {
			require.False(t, nodesToExit[piece.StorageNode])
		}
	})
}

// getRemoteSegment returns a remote pointer its path from satellite.
// nolint:golint
func getRemoteSegment(
	ctx context.Context, t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName string,
) (_ metabase.Segment, key metabase.SegmentKey) {
	t.Helper()

	objects, err := satellite.Metainfo.Metabase.TestingAllObjects(ctx)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	segments, err := satellite.Metainfo.Metabase.TestingAllSegments(ctx)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	require.False(t, segments[0].Inline())

	return segments[0], metabase.SegmentLocation{
		ProjectID:  projectID,
		BucketName: bucketName,
		ObjectKey:  objects[0].ObjectKey,
		Position:   segments[0].Position,
	}.Encode()
}

// corruptPieceData manipulates piece data on a storage node.
func corruptPieceData(ctx context.Context, t *testing.T, planet *testplanet.Planet, corruptedNode *testplanet.StorageNode, corruptedPieceID storj.PieceID) {
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
