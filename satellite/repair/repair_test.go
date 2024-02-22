// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repair_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/testblobs"
	"storj.io/uplink/private/eestream"
	"storj.io/uplink/private/piecestore"
)

// TestDataRepair does the following:
//   - Uploads test data
//   - Kills some nodes and disqualifies 1
//   - Triggers data repair, which repairs the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Shuts down several nodes, but keeping up a number equal to the minim
//     threshold
//   - Downloads the data from those left nodes and check that it's the same than the uploaded one.
func TestDataRepairInMemoryBlake(t *testing.T) {
	testDataRepair(t, true, pb.PieceHashAlgorithm_BLAKE3)
}

func TestDataRepairToDiskSHA256(t *testing.T) {
	testDataRepair(t, false, pb.PieceHashAlgorithm_SHA256)
}

func testDataRepair(t *testing.T, inMemoryRepair bool, hashAlgo pb.PieceHashAlgorithm) {
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

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		for _, storageNode := range planet.StorageNodes {
			storageNode.Storage2.Orders.Sender.Pause()
		}

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(piecestore.WithPieceHashAlgo(ctx, hashAlgo), satellite, "testbucket", "test/path", testData)
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
				_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, node.ID(), time.Now(), overlay.DisqualificationReasonUnknown)
				require.NoError(t, err)
				continue
			}
			if nodesToKill[node.ID()] {
				require.NoError(t, planet.StopNodeAndUpdate(ctx, node))
			}
		}

		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
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

		{
			// test that while repair, order limits without specified bucket are counted correctly
			// for storage node repair bandwidth usage and the storage nodes will be paid for that

			require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))
			for _, storageNode := range planet.StorageNodes {
				storageNode.Storage2.Orders.SendOrders(ctx, time.Now().Add(24*time.Hour))
			}
			repairSettled := make(map[storj.NodeID]uint64)
			err = satellite.DB.StoragenodeAccounting().GetBandwidthSince(ctx, time.Time{}, func(c context.Context, sbr *accounting.StoragenodeBandwidthRollup) error {
				if sbr.Action == uint(pb.PieceAction_GET_REPAIR) {
					repairSettled[sbr.NodeID] += sbr.Settled
				}
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, minThreshold, len(repairSettled))

			for _, value := range repairSettled {
				// TODO verify node ids
				require.NotZero(t, value)
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)
	})
}

// TestDataRepairPendingObject does the following:
//   - Starts new multipart upload with one part of test data. Does not complete the multipart upload.
//   - Kills some nodes and disqualifies 1
//   - Triggers data repair, which repairs the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Shuts down several nodes, but keeping up a number equal to the minim
//     threshold
//   - Completes the multipart upload.
//   - Downloads the data from those left nodes and check that it's the same than the uploaded one.
func TestDataRepairPendingObject(t *testing.T) {
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
					config.Repairer.InMemoryRepair = true
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

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
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
				_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, node.ID(), time.Now(), overlay.DisqualificationReasonUnknown)
				require.NoError(t, err)
				continue
			}
			if nodesToKill[node.ID()] {
				require.NoError(t, planet.StopNodeAndUpdate(ctx, node))
			}
		}

		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
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

// TestMinRequiredDataRepair does the following:
//   - Uploads test data
//   - Kills all but the minimum number of nodes carrying the uploaded segment
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair succeed.
//     Reputation info to be updated for all remaining nodes.
func TestMinRequiredDataRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 15,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
					config.Repairer.ReputationUpdateEnabled = true
					config.Reputation.InitialAlpha = 1
					config.Reputation.InitialBeta = 0.01
					config.Reputation.AuditLambda = 0.95
				},
				testplanet.ReconfigureRS(4, 4, 9, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 9, len(segment.Pieces))
		require.Equal(t, 4, int(segment.Redundancy.RequiredShares))
		toKill := 5

		// kill nodes and track lost pieces
		var availableNodes storj.NodeIDList
		var killedNodes storj.NodeIDList

		for i, piece := range segment.Pieces {
			if i >= toKill {
				availableNodes = append(availableNodes, piece.StorageNode)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
			killedNodes = append(killedNodes, piece.StorageNode)
		}
		require.Equal(t, 4, len(availableNodes))

		// Here we use a different reputation service from the one the
		// repairer is reporting to. To get correct results in a short
		// amount of time, we have to flush all cached node info using
		// TestFlushAllNodeInfo(), below.
		reputationService := planet.Satellites[0].Reputation.Service

		nodesReputation := make(map[storj.NodeID]reputation.Info)
		for _, nodeID := range availableNodes {
			info, err := reputationService.Get(ctx, nodeID)
			require.NoError(t, err)
			nodesReputation[nodeID] = *info
		}

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()
		err = satellite.Repairer.Reputation.TestFlushAllNodeInfo(ctx)
		require.NoError(t, err)
		err = reputationService.TestFlushAllNodeInfo(ctx)
		require.NoError(t, err)

		for _, nodeID := range availableNodes {
			info, err := reputationService.Get(ctx, nodeID)
			require.NoError(t, err)

			infoBefore := nodesReputation[nodeID]
			require.Equal(t, infoBefore.TotalAuditCount+1, info.TotalAuditCount)
			require.Equal(t, infoBefore.AuditSuccessCount+1, info.AuditSuccessCount)
			require.Greater(t, reputationRatio(*info), reputationRatio(infoBefore))
		}

		// repair succeed, so segment should not contain any killed node
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		for _, piece := range segmentAfter.Pieces {
			require.NotContains(t, killedNodes, piece.StorageNode, "there should be no killed nodes in pointer")
		}
	})
}

// TestFailedDataRepair does the following:
//   - Uploads test data
//   - Kills some nodes carrying the uploaded segment but keep it above minimum requirement
//   - On one of the remaining nodes, return unknown error during downloading of the piece
//   - Stop one of the remaining nodes, for it to be offline during repair
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair failed and the pointer was not updated.
//     Reputation info to be updated for all remaining nodes.
func TestFailedDataRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 15,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
					config.Repairer.ReputationUpdateEnabled = true
					config.Reputation.InitialAlpha = 1
					config.Reputation.InitialBeta = 0.01
					config.Reputation.AuditLambda = 0.95
				},
				testplanet.ReconfigureRS(4, 5, 9, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 9, len(segment.Pieces))
		require.Equal(t, 4, int(segment.Redundancy.RequiredShares))
		toKill := 4

		// kill nodes and track lost pieces
		var availablePieces metabase.Pieces
		var originalNodes storj.NodeIDList

		for i, piece := range segment.Pieces {
			originalNodes = append(originalNodes, piece.StorageNode)
			if i >= toKill {
				availablePieces = append(availablePieces, piece)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.Equal(t, 5, len(availablePieces))

		// choose first piece for shutting down node, for it to always be in the first limiter batch
		offlinePiece := availablePieces[0]
		// choose last piece for bad node, for it to always be in the last limiter batch
		unknownPiece := availablePieces[4]

		// stop offline node
		offlineNode := planet.FindNode(offlinePiece.StorageNode)
		require.NotNil(t, offlineNode)
		require.NoError(t, planet.StopPeer(offlineNode))

		// set unknown error for download from bad node
		badNode := planet.FindNode(unknownPiece.StorageNode)
		require.NotNil(t, badNode)
		badNodeDB := badNode.DB.(*testblobs.BadDB)
		badNodeDB.SetError(errs.New("unknown error"))

		reputationService := satellite.Repairer.Reputation

		nodesReputation := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputation[piece.StorageNode] = *info
		}

		satellite.Repair.Repairer.TestingSetMinFailures(2) // expecting one erroring node, one offline node
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		nodesReputationAfter := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputationAfter[piece.StorageNode] = *info
		}

		// repair shouldn't update audit status
		for _, piece := range availablePieces {
			successfulNodeReputation := nodesReputation[piece.StorageNode]
			successfulNodeReputationAfter := nodesReputationAfter[piece.StorageNode]
			require.Equal(t, successfulNodeReputation.TotalAuditCount, successfulNodeReputationAfter.TotalAuditCount)
			require.Equal(t, successfulNodeReputation.AuditSuccessCount, successfulNodeReputationAfter.AuditSuccessCount)
			require.Equal(t, successfulNodeReputation.AuditReputationAlpha, successfulNodeReputationAfter.AuditReputationAlpha)
			require.Equal(t, successfulNodeReputation.AuditReputationBeta, successfulNodeReputationAfter.AuditReputationBeta)
		}

		// repair should fail, so segment should contain all the original nodes
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		for _, piece := range segmentAfter.Pieces {
			require.Contains(t, originalNodes, piece.StorageNode, "there should be no new nodes in pointer")
		}
	})
}

// TestOfflineNodeDataRepair does the following:
//   - Uploads test data
//   - Kills some nodes carrying the uploaded segment but keep it above minimum requirement
//   - Stop one of the remaining nodes, for it to be offline during repair
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair succeed and the pointer should contain the offline piece.
//     Reputation info to be updated for all remaining nodes.
func TestOfflineNodeDataRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 15,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
					config.Repairer.ReputationUpdateEnabled = true
					config.Reputation.InitialAlpha = 1
					config.Reputation.InitialBeta = 0.01
					config.Reputation.AuditLambda = 0.95
				},
				testplanet.ReconfigureRS(3, 4, 9, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 9, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 5

		// kill nodes and track lost pieces
		var availablePieces metabase.Pieces
		var killedNodes storj.NodeIDList

		for i, piece := range segment.Pieces {
			if i >= toKill {
				availablePieces = append(availablePieces, piece)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
			killedNodes = append(killedNodes, piece.StorageNode)
		}
		require.Equal(t, 4, len(availablePieces))
		require.Equal(t, 5, len(killedNodes))

		// choose first piece for shutting down node, for it to always be in the first limiter batch
		offlinePiece := availablePieces[0]

		// stop offline node
		offlineNode := planet.FindNode(offlinePiece.StorageNode)
		require.NotNil(t, offlineNode)
		require.NoError(t, planet.StopPeer(offlineNode))

		reputationService := satellite.Repairer.Reputation

		nodesReputation := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputation[piece.StorageNode] = *info
		}

		satellite.Repair.Repairer.TestingSetMinFailures(1) // expect one offline node
		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		nodesReputationAfter := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputationAfter[piece.StorageNode] = *info
		}

		// repair should update audit status
		for _, piece := range availablePieces[1:] {
			successfulNodeReputation := nodesReputation[piece.StorageNode]
			successfulNodeReputationAfter := nodesReputationAfter[piece.StorageNode]
			require.Equal(t, successfulNodeReputation.TotalAuditCount+1, successfulNodeReputationAfter.TotalAuditCount)
			require.Equal(t, successfulNodeReputation.AuditSuccessCount+1, successfulNodeReputationAfter.AuditSuccessCount)
			require.Greater(t, reputationRatio(successfulNodeReputationAfter), reputationRatio(successfulNodeReputation))
		}

		offlineNodeReputation := nodesReputation[offlinePiece.StorageNode]
		offlineNodeReputationAfter := nodesReputationAfter[offlinePiece.StorageNode]
		require.Equal(t, offlineNodeReputation.TotalAuditCount+1, offlineNodeReputationAfter.TotalAuditCount)
		require.Equal(t, int32(0), offlineNodeReputationAfter.AuditHistory.Windows[0].OnlineCount)

		// repair succeed, so segment should not contain any killed node
		// offline node's piece should still exists
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Contains(t, segmentAfter.Pieces, offlinePiece, "offline piece should still be in segment")
		for _, piece := range segmentAfter.Pieces {
			require.NotContains(t, killedNodes, piece.StorageNode, "there should be no killed nodes in pointer")
		}
	})
}

// TestUnknownErrorDataRepair does the following:
//   - Uploads test data
//   - Kills some nodes carrying the uploaded segment but keep it above minimum requirement
//   - On one of the remaining nodes, return unknown error during downloading of the piece
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair succeed and the pointer should contain the unknown piece.
//     Reputation info to be updated for all remaining nodes.
func TestUnknownErrorDataRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 15,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
					config.Repairer.ReputationUpdateEnabled = true
					config.Reputation.InitialAlpha = 1
					config.Reputation.InitialBeta = 0.01
					config.Reputation.AuditLambda = 0.95
				},
				testplanet.ReconfigureRS(3, 4, 9, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 9, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 5

		// kill nodes and track lost pieces
		var availablePieces metabase.Pieces
		var killedNodes storj.NodeIDList

		for i, piece := range segment.Pieces {
			if i >= toKill {
				availablePieces = append(availablePieces, piece)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
			killedNodes = append(killedNodes, piece.StorageNode)
		}
		require.Equal(t, 4, len(availablePieces))
		require.Equal(t, 5, len(killedNodes))

		// choose first piece for corruption, for it to always be in the first limiter batch
		unknownPiece := availablePieces[0]

		// set unknown error for download from bad node
		badNode := planet.FindNode(unknownPiece.StorageNode)
		require.NotNil(t, badNode)
		badNodeDB := badNode.DB.(*testblobs.BadDB)
		badNodeDB.SetError(errs.New("unknown error"))

		reputationService := satellite.Repairer.Reputation

		nodesReputation := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputation[piece.StorageNode] = *info
		}

		satellite.Repair.Repairer.TestingSetMinFailures(1) // expecting one bad node
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		nodesReputationAfter := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputationAfter[piece.StorageNode] = *info
		}

		// repair should update audit status
		for _, piece := range availablePieces[1:] {
			successfulNodeReputation := nodesReputation[piece.StorageNode]
			successfulNodeReputationAfter := nodesReputationAfter[piece.StorageNode]
			require.Equal(t, successfulNodeReputation.TotalAuditCount+1, successfulNodeReputationAfter.TotalAuditCount)
			require.Equal(t, successfulNodeReputation.AuditSuccessCount+1, successfulNodeReputationAfter.AuditSuccessCount)
			require.Greater(t, reputationRatio(successfulNodeReputationAfter), reputationRatio(successfulNodeReputation))
		}

		badNodeReputation := nodesReputation[unknownPiece.StorageNode]
		badNodeReputationAfter := nodesReputationAfter[unknownPiece.StorageNode]
		require.Equal(t, badNodeReputation.TotalAuditCount+1, badNodeReputationAfter.TotalAuditCount)
		require.Less(t, badNodeReputation.UnknownAuditReputationBeta, badNodeReputationAfter.UnknownAuditReputationBeta)
		require.GreaterOrEqual(t, badNodeReputation.UnknownAuditReputationAlpha, badNodeReputationAfter.UnknownAuditReputationAlpha)

		// repair succeed, so segment should not contain any killed node
		// unknown piece should still exists
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Contains(t, segmentAfter.Pieces, unknownPiece, "unknown piece should still be in segment")
		for _, piece := range segmentAfter.Pieces {
			require.NotContains(t, killedNodes, piece.StorageNode, "there should be no killed nodes in pointer")
		}
	})
}

// TestMissingPieceDataRepair_Succeed does the following:
//   - Uploads test data
//   - Kills some nodes carrying the uploaded segment but keep it above minimum requirement
//   - On one of the remaining nodes, delete the piece data being stored by that node
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair succeed and the pointer should not contain the missing piece.
//     Reputation info to be updated for all remaining nodes.
func TestMissingPieceDataRepair_Succeed(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 15,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
					config.Repairer.ReputationUpdateEnabled = true
					config.Reputation.InitialAlpha = 1
					config.Reputation.InitialBeta = 0.01
					config.Reputation.AuditLambda = 0.95
				},
				testplanet.ReconfigureRS(3, 4, 9, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 9, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 5

		// kill nodes and track lost pieces
		var availablePieces metabase.Pieces

		for i, piece := range segment.Pieces {
			if i >= toKill {
				availablePieces = append(availablePieces, piece)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.Equal(t, 4, len(availablePieces))

		// choose first piece for deletion, for it to always be in the first limiter batch
		missingPiece := availablePieces[0]

		// delete piece
		missingPieceNode := planet.FindNode(missingPiece.StorageNode)
		require.NotNil(t, missingPieceNode)
		pieceID := segment.RootPieceID.Derive(missingPiece.StorageNode, int32(missingPiece.Number))
		err = missingPieceNode.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		reputationService := satellite.Repairer.Reputation

		nodesReputation := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputation[piece.StorageNode] = *info
		}

		satellite.Repair.Repairer.TestingSetMinFailures(1) // expect one node to have a missing piece
		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		nodesReputationAfter := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputationAfter[piece.StorageNode] = *info
		}

		// repair should update audit status
		for _, piece := range availablePieces[1:] {
			successfulNodeReputation := nodesReputation[piece.StorageNode]
			successfulNodeReputationAfter := nodesReputationAfter[piece.StorageNode]
			require.Equal(t, successfulNodeReputation.TotalAuditCount+1, successfulNodeReputationAfter.TotalAuditCount)
			require.Equal(t, successfulNodeReputation.AuditSuccessCount+1, successfulNodeReputationAfter.AuditSuccessCount)
			require.Greater(t, reputationRatio(successfulNodeReputationAfter), reputationRatio(successfulNodeReputation))
		}

		missingPieceNodeReputation := nodesReputation[missingPiece.StorageNode]
		missingPieceNodeReputationAfter := nodesReputationAfter[missingPiece.StorageNode]
		require.Equal(t, missingPieceNodeReputation.TotalAuditCount+1, missingPieceNodeReputationAfter.TotalAuditCount)
		require.Less(t, reputationRatio(missingPieceNodeReputationAfter), reputationRatio(missingPieceNodeReputation))

		// repair succeeded, so segment should not contain missing piece
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		for _, piece := range segmentAfter.Pieces {
			require.NotEqual(t, piece.Number, missingPiece.Number, "there should be no missing piece in pointer")
		}
	})
}

// TestMissingPieceDataRepair_Failed does the following:
//   - Uploads test data
//   - Kills all but the minimum number of nodes carrying the uploaded segment
//   - On one of the remaining nodes, delete the piece data being stored by that node
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair failed and the pointer was not updated.
//     Reputation info to be updated for node missing the piece.
func TestMissingPieceDataRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 15,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
					config.Repairer.ReputationUpdateEnabled = true
					config.Reputation.InitialAlpha = 1
					config.Reputation.AuditLambda = 0.95
				},
				testplanet.ReconfigureRS(4, 4, 9, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 9, len(segment.Pieces))
		require.Equal(t, 4, int(segment.Redundancy.RequiredShares))
		toKill := 5

		// kill nodes and track lost pieces
		originalNodes := make(map[storj.NodeID]bool)
		var availablePieces metabase.Pieces

		for i, piece := range segment.Pieces {
			originalNodes[piece.StorageNode] = true
			if i >= toKill {
				availablePieces = append(availablePieces, piece)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.Equal(t, 4, len(availablePieces))

		missingPiece := availablePieces[0]

		// delete piece
		missingPieceNode := planet.FindNode(missingPiece.StorageNode)
		require.NotNil(t, missingPieceNode)
		pieceID := segment.RootPieceID.Derive(missingPiece.StorageNode, int32(missingPiece.Number))
		err = missingPieceNode.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		reputationService := satellite.Repairer.Reputation

		nodesReputation := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputation[piece.StorageNode] = *info
		}

		var successful []repairer.PieceFetchResult
		satellite.Repairer.SegmentRepairer.OnTestingPiecesReportHook = func(pieces repairer.FetchResultReport) {
			successful = pieces.Successful
		}

		satellite.Repair.Repairer.TestingSetMinFailures(1) // expect one missing piece
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		nodesReputationAfter := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputationAfter[piece.StorageNode] = *info
		}

		// repair shouldn't update audit status
		for _, result := range successful {
			successfulNodeReputation := nodesReputation[result.Piece.StorageNode]
			successfulNodeReputationAfter := nodesReputationAfter[result.Piece.StorageNode]
			require.Equal(t, successfulNodeReputation.TotalAuditCount, successfulNodeReputationAfter.TotalAuditCount)
			require.Equal(t, successfulNodeReputation.AuditSuccessCount, successfulNodeReputationAfter.AuditSuccessCount)
			require.Equal(t, successfulNodeReputation.AuditReputationAlpha, successfulNodeReputationAfter.AuditReputationAlpha)
			require.Equal(t, successfulNodeReputation.AuditReputationBeta, successfulNodeReputationAfter.AuditReputationBeta)
		}

		// repair should fail, so segment should contain all the original nodes
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		for _, piece := range segmentAfter.Pieces {
			require.Contains(t, originalNodes, piece.StorageNode, "there should be no new nodes in pointer")
		}
	})
}

// TestCorruptDataRepair_Succeed does the following:
//   - Uploads test data using different hash algorithms (Blake3 and SHA256)
//   - Kills some nodes carrying the uploaded segment but keep it above minimum requirement
//   - On one of the remaining nodes, corrupt the piece data being stored by that node
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair succeed and the pointer should not contain the corrupted piece.
//     Reputation info to be updated for all remaining nodes.
func TestCorruptDataRepair_Succeed(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	for _, tt := range []struct {
		name     string
		hashAlgo pb.PieceHashAlgorithm
	}{
		{
			name:     "BLAKE3",
			hashAlgo: pb.PieceHashAlgorithm_BLAKE3,
		},
		{
			name:     "SHA256",
			hashAlgo: pb.PieceHashAlgorithm_SHA256,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			testplanet.Run(t, testplanet.Config{
				SatelliteCount:   1,
				StorageNodeCount: 15,
				UplinkCount:      1,
				Reconfigure: testplanet.Reconfigure{
					Satellite: testplanet.Combine(
						func(log *zap.Logger, index int, config *satellite.Config) {
							config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
							config.Repairer.InMemoryRepair = true
							config.Repairer.ReputationUpdateEnabled = true
							config.Reputation.InitialAlpha = 1
							config.Reputation.AuditLambda = 0.95
						},
						testplanet.ReconfigureRS(3, 4, 9, 9),
					),
				},
			}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
				uplinkPeer := planet.Uplinks[0]
				satellite := planet.Satellites[0]
				// stop audit to prevent possible interactions i.e. repair timeout problems
				satellite.Audit.Worker.Loop.Pause()

				satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
				satellite.Repair.Repairer.Loop.Pause()

				var testData = testrand.Bytes(8 * memory.KiB)
				// first, upload some remote data
				err := uplinkPeer.Upload(piecestore.WithPieceHashAlgo(ctx, tt.hashAlgo), satellite, "testbucket", "test/path", testData)
				require.NoError(t, err)

				segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
				require.Equal(t, 9, len(segment.Pieces))
				require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
				toKill := 5

				// kill nodes and track lost pieces
				var availablePieces metabase.Pieces

				for i, piece := range segment.Pieces {
					if i >= toKill {
						availablePieces = append(availablePieces, piece)
						continue
					}

					err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
					require.NoError(t, err)
				}
				require.Equal(t, 4, len(availablePieces))

				// choose first piece for corruption, for it to always be in the first limiter batch
				corruptedPiece := availablePieces[0]

				// corrupt piece data
				corruptedNode := planet.FindNode(corruptedPiece.StorageNode)
				require.NotNil(t, corruptedNode)
				corruptedPieceID := segment.RootPieceID.Derive(corruptedPiece.StorageNode, int32(corruptedPiece.Number))
				corruptPieceData(ctx, t, planet, corruptedNode, corruptedPieceID)

				reputationService := satellite.Repairer.Reputation

				nodesReputation := make(map[storj.NodeID]reputation.Info)
				for _, piece := range availablePieces {
					info, err := reputationService.Get(ctx, piece.StorageNode)
					require.NoError(t, err)
					nodesReputation[piece.StorageNode] = *info
				}

				satellite.Repair.Repairer.TestingSetMinFailures(1) // expect one node with bad data
				// trigger checker with ranged loop to add segment to repair queue
				_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
				require.NoError(t, err)
				satellite.Repair.Repairer.Loop.Restart()
				satellite.Repair.Repairer.Loop.TriggerWait()
				satellite.Repair.Repairer.Loop.Pause()
				satellite.Repair.Repairer.WaitForPendingRepairs()

				nodesReputationAfter := make(map[storj.NodeID]reputation.Info)
				for _, piece := range availablePieces {
					info, err := reputationService.Get(ctx, piece.StorageNode)
					require.NoError(t, err)
					nodesReputationAfter[piece.StorageNode] = *info
				}

				// repair should update audit status
				for _, piece := range availablePieces[1:] {
					successfulNodeReputation := nodesReputation[piece.StorageNode]
					successfulNodeReputationAfter := nodesReputationAfter[piece.StorageNode]
					require.Equal(t, successfulNodeReputation.TotalAuditCount+1, successfulNodeReputationAfter.TotalAuditCount)
					require.Equal(t, successfulNodeReputation.AuditSuccessCount+1, successfulNodeReputationAfter.AuditSuccessCount)
					require.GreaterOrEqual(t, reputationRatio(successfulNodeReputationAfter), reputationRatio(successfulNodeReputation))
				}

				corruptedNodeReputation := nodesReputation[corruptedPiece.StorageNode]
				corruptedNodeReputationAfter := nodesReputationAfter[corruptedPiece.StorageNode]
				require.Equal(t, corruptedNodeReputation.TotalAuditCount+1, corruptedNodeReputationAfter.TotalAuditCount)
				require.Less(t, reputationRatio(corruptedNodeReputationAfter), reputationRatio(corruptedNodeReputation))

				// repair succeeded, so segment should not contain corrupted piece
				segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
				for _, piece := range segmentAfter.Pieces {
					require.NotEqual(t, piece.Number, corruptedPiece.Number, "there should be no corrupted piece in pointer")
				}
			})
		})
	}
}

// TestCorruptDataRepair_Failed does the following:
//   - Uploads test data
//   - Kills all but the minimum number of nodes carrying the uploaded segment
//   - On one of the remaining nodes, corrupt the piece data being stored by that node
//   - Triggers data repair, which attempts to repair the data from the remaining nodes to
//     the numbers of nodes determined by the upload repair max threshold
//   - Expects that the repair failed and the pointer was not updated.
//     Reputation info to be updated for corrupted node.
func TestCorruptDataRepair_Failed(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 15,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
					config.Repairer.ReputationUpdateEnabled = true
					config.Reputation.InitialAlpha = 1
					config.Reputation.AuditLambda = 0.95
				},
				testplanet.ReconfigureRS(4, 4, 9, 9),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 9, len(segment.Pieces))
		require.Equal(t, 4, int(segment.Redundancy.RequiredShares))
		toKill := 5

		// kill nodes and track lost pieces
		originalNodes := make(map[storj.NodeID]bool)
		var availablePieces metabase.Pieces

		for i, piece := range segment.Pieces {
			originalNodes[piece.StorageNode] = true
			if i >= toKill {
				availablePieces = append(availablePieces, piece)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.Equal(t, 4, len(availablePieces))

		corruptedPiece := availablePieces[0]

		// corrupt piece data
		corruptedNode := planet.FindNode(corruptedPiece.StorageNode)
		require.NotNil(t, corruptedNode)
		corruptedPieceID := segment.RootPieceID.Derive(corruptedPiece.StorageNode, int32(corruptedPiece.Number))
		corruptPieceData(ctx, t, planet, corruptedNode, corruptedPieceID)

		reputationService := satellite.Repairer.Reputation

		nodesReputation := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputation[piece.StorageNode] = *info
		}

		var successful []repairer.PieceFetchResult
		satellite.Repairer.SegmentRepairer.OnTestingPiecesReportHook = func(report repairer.FetchResultReport) {
			successful = report.Successful
		}

		satellite.Repair.Repairer.TestingSetMinFailures(1) // expect one corrupted piece
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		nodesReputationAfter := make(map[storj.NodeID]reputation.Info)
		for _, piece := range availablePieces {
			info, err := reputationService.Get(ctx, piece.StorageNode)
			require.NoError(t, err)
			nodesReputationAfter[piece.StorageNode] = *info
		}

		// repair shouldn't update audit status
		for _, result := range successful {
			successfulNodeReputation := nodesReputation[result.Piece.StorageNode]
			successfulNodeReputationAfter := nodesReputationAfter[result.Piece.StorageNode]
			require.Equal(t, successfulNodeReputation.TotalAuditCount, successfulNodeReputationAfter.TotalAuditCount)
			require.Equal(t, successfulNodeReputation.AuditSuccessCount, successfulNodeReputationAfter.AuditSuccessCount)
			require.Equal(t, successfulNodeReputation.AuditReputationAlpha, successfulNodeReputationAfter.AuditReputationAlpha)
			require.Equal(t, successfulNodeReputation.AuditReputationBeta, successfulNodeReputationAfter.AuditReputationBeta)
		}

		// repair should fail, so segment should contain all the original nodes
		segmentAfter, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		for _, piece := range segmentAfter.Pieces {
			require.Contains(t, originalNodes, piece.StorageNode, "there should be no new nodes in pointer")
		}
	})
}

// TestRepairExpiredSegment
// - Upload tests data to 7 nodes
// - Kill nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Modify segment to be expired
// - Run the repairer
// - Verify segment is no longer in the repair queue.
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
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
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
			_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, nodeID, time.Now(), overlay.DisqualificationReasonUnknown)
			require.NoError(t, err)

		}

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

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

		// Verify that the segment is not still in the queue
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
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

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
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
			_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, nodeID, time.Now(), overlay.DisqualificationReasonUnknown)
			require.NoError(t, err)

		}

		// trigger checker to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

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

// TestSegmentDeletedDuringRepair
// - Upload tests data to 7 nodes
// - Kill nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Delete segment from the satellite database when repair is in progress.
// - Run the repairer
// - Verify segment is no longer in the repair queue.
// - Verify no audit has been recorded.
func TestSegmentDeletedDuringRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
				},
				testplanet.ReconfigureRS(3, 4, 6, 6),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 6, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 3

		// kill nodes and track lost pieces
		var availableNodes storj.NodeIDList

		for i, piece := range segment.Pieces {
			if i >= toKill {
				availableNodes = append(availableNodes, piece.StorageNode)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.Equal(t, 3, len(availableNodes))

		// trigger checker to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// delete segment
		satellite.Repairer.SegmentRepairer.OnTestingCheckSegmentAlteredHook = func() {
			err = uplinkPeer.DeleteObject(ctx, satellite, "testbucket", "test/path")
			require.NoError(t, err)

		}

		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// Verify that no audit has been recorded for participated nodes.
		reputationService := satellite.Reputation.Service

		for _, nodeID := range availableNodes {
			info, err := reputationService.Get(ctx, nodeID)
			require.NoError(t, err)
			require.Equal(t, int64(0), info.TotalAuditCount)
		}
	})
}

// TestSegmentModifiedDuringRepair
// - Upload tests data to 7 nodes
// - Kill nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Modify segment when repair is in progress.
// - Run the repairer
// - Verify segment is no longer in the repair queue.
// - Verify no audit has been recorded.
func TestSegmentModifiedDuringRepair(t *testing.T) {
	const RepairMaxExcessRateOptimalThreshold = 0.05

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 10,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.MaxExcessRateOptimalThreshold = RepairMaxExcessRateOptimalThreshold
					config.Repairer.InMemoryRepair = true
				},
				testplanet.ReconfigureRS(3, 4, 6, 6),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 6, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 3

		// kill nodes and track lost pieces
		var availableNodes storj.NodeIDList

		for i, piece := range segment.Pieces {
			if i >= toKill {
				availableNodes = append(availableNodes, piece.StorageNode)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.Equal(t, 3, len(availableNodes))

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// delete segment
		satellite.Repairer.SegmentRepairer.OnTestingCheckSegmentAlteredHook = func() {
			// remove one piece from the segment so that checkIfSegmentAltered fails
			err = satellite.Metabase.DB.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
				StreamID:      segment.StreamID,
				Position:      segment.Position,
				OldPieces:     segment.Pieces,
				NewPieces:     append([]metabase.Piece{segment.Pieces[0]}, segment.Pieces[2:]...),
				NewRedundancy: segment.Redundancy,
			})
			require.NoError(t, err)
		}

		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// Verify that no audit has been recorded for participated nodes.
		reputationService := satellite.Reputation.Service

		for _, nodeID := range availableNodes {
			info, err := reputationService.Get(ctx, nodeID)
			require.NoError(t, err)
			require.Equal(t, int64(0), info.TotalAuditCount)
		}
	})
}

// TestIrreparableSegmentAccordingToOverlay
// - Upload tests data to 7 nodes
// - Disqualify nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Disqualify nodes so that online nodes < minimum threshold
// - Run the repairer
// - Verify segment is still in the repair queue.
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

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		// dq 3 nodes so that pointer has 4 left (less than repair threshold)
		toDQ := 3
		remotePieces := segment.Pieces

		for i := 0; i < toDQ; i++ {
			_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, remotePieces[i].StorageNode, time.Now(), overlay.DisqualificationReasonUnknown)
			require.NoError(t, err)
		}

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// Disqualify nodes so that online nodes < minimum threshold
		// This will make the segment irreparable
		for _, piece := range remotePieces {
			_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, piece.StorageNode, time.Now(), overlay.DisqualificationReasonUnknown)
			require.NoError(t, err)
		}

		// Verify that the segment is on the repair queue
		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 1)

		// Run the repairer
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the irreparable segment is still in repair queue
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, count, 1)
	})
}

// TestIrreparableSegmentNodesOffline
// - Upload tests data to 7 nodes
// - Disqualify nodes so that repair threshold > online nodes > minimum threshold
// - Call checker to add segment to the repair queue
// - Kill (as opposed to disqualifying) nodes so that online nodes < minimum threshold
// - Run the repairer
// - Verify segment is still in the repair queue.
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

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")

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

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

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

		// Run the repairer
		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the irreparable segment is still in repair queue
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
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
		LastNet:    local.Address,
		IsUp:       isUp,
		Operator:   &local.Operator,
		Capacity:   &local.Capacity,
		Version:    &local.Version,
	}
	return overlayDB.UpdateCheckIn(ctx, checkInInfo, time.Now().Add(-24*time.Hour), overlay.NodeSelectionConfig{})
}

// TestRepairMultipleDisqualifiedAndSuspended does the following:
// - Uploads test data to 7 nodes
// - Disqualifies 2 nodes and suspends 1 node
// - Triggers data repair, which repairs the data from the remaining 4 nodes to additional 3 new nodes
// - Shuts down the 4 nodes from which the data was repaired
// - Now we have just the 3 new nodes to which the data was repaired
// - Downloads the data from these 3 nodes (succeeds because 3 nodes are enough for download)
// - Expect newly repaired pointer does not contain the disqualified or suspended nodes.
func TestRepairMultipleDisqualifiedAndSuspended(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 12,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = true
				},
				testplanet.ReconfigureRS(3, 5, 7, 7),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from metainfo
		segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
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
			_, err := satellite.DB.OverlayCache().DisqualifyNode(ctx, remotePieces[i].StorageNode, time.Now(), overlay.DisqualificationReasonUnknown)
			require.NoError(t, err)
		}
		for i := toDisqualify; i < toDisqualify+toSuspend; i++ {
			nodesToSuspend[remotePieces[i].StorageNode] = true
			err := satellite.DB.OverlayCache().TestSuspendNodeUnknownAudit(ctx, remotePieces[i].StorageNode, time.Now())
			require.NoError(t, err)
		}
		for i := toDisqualify + toSuspend; i < len(remotePieces); i++ {
			nodesToKeepAlive[remotePieces[i].StorageNode] = true
		}

		err = satellite.RangedLoop.Repair.Observer.RefreshReliabilityCache(ctx)
		require.NoError(t, err)

		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
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

		segments, err = satellite.Metabase.DB.TestingAllSegments(ctx)
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
//     the numbers of nodes determined by the upload repair max threshold
func TestDataRepairOverride_HigherLimit(t *testing.T) {
	const repairOverride = 6

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = true
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

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
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

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
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
//     the numbers of nodes determined by the upload repair max threshold
func TestDataRepairOverride_LowerLimit(t *testing.T) {
	const repairOverride = 4

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = true
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

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
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

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

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

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

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
func TestDataRepairUploadLimit(t *testing.T) {
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
					config.Repairer.InMemoryRepair = true
				},
				testplanet.ReconfigureRS(3, repairThreshold, successThreshold, maxThreshold),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
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

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
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
func TestRepairGracefullyExited(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 12,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = true
				},
				testplanet.ReconfigureRS(3, 5, 7, 7),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
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

		err = satellite.RangedLoop.Repair.Observer.RefreshReliabilityCache(ctx)
		require.NoError(t, err)

		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)
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
//
//nolint:golint
func getRemoteSegment(
	ctx context.Context, t *testing.T, satellite *testplanet.Satellite, projectID uuid.UUID, bucketName string,
) (_ metabase.Segment, key metabase.SegmentKey) {
	t.Helper()

	objects, err := satellite.Metabase.DB.TestingAllObjects(ctx)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	segments, err := satellite.Metabase.DB.TestingAllSegments(ctx)
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

	blobRef := blobstore.BlobRef{
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

type mockConnector struct {
	realConnector   rpc.Connector
	addressesDialed []string
	dialInstead     map[string]string
}

func (m *mockConnector) DialContext(ctx context.Context, tlsConfig *tls.Config, address string) (rpc.ConnectorConn, error) {
	m.addressesDialed = append(m.addressesDialed, address)
	replacement := m.dialInstead[address]
	if replacement == "" {
		// allow numeric ip addresses through, return errors for unexpected dns lookups
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if net.ParseIP(host) == nil {
			return nil, &net.DNSError{
				Err:        "unexpected lookup",
				Name:       address,
				Server:     "a.totally.real.dns.server.i.promise",
				IsNotFound: true,
			}
		}
		replacement = address
	}
	return m.realConnector.DialContext(ctx, tlsConfig, replacement)
}

func ecRepairerWithMockConnector(t testing.TB, sat *testplanet.Satellite, mock *mockConnector) *repairer.ECRepairer {
	tlsOptions := sat.Dialer.TLSOptions
	newDialer := rpc.NewDefaultDialer(tlsOptions)
	mock.realConnector = newDialer.Connector
	newDialer.Connector = mock

	ec := repairer.NewECRepairer(
		zaptest.NewLogger(t).Named("a-special-repairer"),
		newDialer,
		signing.SigneeFromPeerIdentity(sat.Identity.PeerIdentity()),
		sat.Config.Repairer.DialTimeout,
		sat.Config.Repairer.DownloadTimeout,
		sat.Config.Repairer.InMemoryRepair,
		sat.Config.Repairer.InMemoryUpload,
	)
	return ec
}

func TestECRepairerGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 3, 6, 6),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		ecRepairer := satellite.Repairer.EcRepairer

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)
		getOrderLimits, getPrivateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		_, piecesReport, err := ecRepairer.Get(ctx, getOrderLimits, cachedIPsAndPorts, getPrivateKey, redundancy, int64(segment.EncryptedSize))
		require.NoError(t, err)
		require.Equal(t, 0, len(piecesReport.Offline))
		require.Equal(t, 0, len(piecesReport.Failed))
		require.Equal(t, 0, len(piecesReport.Contained))
		require.Equal(t, 0, len(piecesReport.Unknown))
		require.Equal(t, int(segment.Redundancy.RequiredShares), len(piecesReport.Successful))
	})
}

func TestECRepairerGetCorrupted(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 3, 6, 6),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 6, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 2

		// kill nodes and track lost pieces
		var corruptedPiece metabase.Piece
		for i, piece := range segment.Pieces {
			if i >= toKill {
				// this means the node will be kept alive for repair
				// choose piece to corrupt
				if corruptedPiece.StorageNode.IsZero() {
					corruptedPiece = piece
				}
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.False(t, corruptedPiece.StorageNode.IsZero())

		// corrupted node
		corruptedNode := planet.FindNode(corruptedPiece.StorageNode)
		require.NotNil(t, corruptedNode)
		pieceID := segment.RootPieceID.Derive(corruptedPiece.StorageNode, int32(corruptedPiece.Number))
		corruptPieceData(ctx, t, planet, corruptedNode, pieceID)

		ecRepairer := satellite.Repairer.EcRepairer

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)
		getOrderLimits, getPrivateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		ecRepairer.TestingSetMinFailures(1)
		_, piecesReport, err := ecRepairer.Get(ctx, getOrderLimits, cachedIPsAndPorts, getPrivateKey, redundancy, int64(segment.EncryptedSize))
		require.NoError(t, err)
		require.Equal(t, 0, len(piecesReport.Offline))
		require.Equal(t, 1, len(piecesReport.Failed))
		require.Equal(t, 0, len(piecesReport.Contained))
		require.Equal(t, 0, len(piecesReport.Unknown))
		require.Equal(t, int(segment.Redundancy.RequiredShares), len(piecesReport.Successful))
		require.Equal(t, corruptedPiece, piecesReport.Failed[0].Piece)
	})
}

func TestECRepairerGetMissingPiece(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 3, 6, 6),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 6, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 2

		// kill nodes and track lost pieces
		var missingPiece metabase.Piece
		for i, piece := range segment.Pieces {
			if i >= toKill {
				// this means the node will be kept alive for repair
				// choose a piece for deletion
				if missingPiece.StorageNode.IsZero() {
					missingPiece = piece
				}
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.False(t, missingPiece.StorageNode.IsZero())

		// delete piece
		node := planet.FindNode(missingPiece.StorageNode)
		require.NotNil(t, node)
		pieceID := segment.RootPieceID.Derive(missingPiece.StorageNode, int32(missingPiece.Number))
		err = node.Storage2.Store.Delete(ctx, satellite.ID(), pieceID)
		require.NoError(t, err)

		ecRepairer := satellite.Repairer.EcRepairer

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)
		getOrderLimits, getPrivateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		ecRepairer.TestingSetMinFailures(1)
		_, piecesReport, err := ecRepairer.Get(ctx, getOrderLimits, cachedIPsAndPorts, getPrivateKey, redundancy, int64(segment.EncryptedSize))
		require.NoError(t, err)
		require.Equal(t, 0, len(piecesReport.Offline))
		require.Equal(t, 1, len(piecesReport.Failed))
		require.Equal(t, 0, len(piecesReport.Contained))
		require.Equal(t, 0, len(piecesReport.Unknown))
		require.Equal(t, int(segment.Redundancy.RequiredShares), len(piecesReport.Successful))
		require.Equal(t, missingPiece, piecesReport.Failed[0].Piece)
	})
}

func TestECRepairerGetOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(3, 3, 6, 6),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 6, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 2

		// kill nodes and track lost pieces
		var offlinePiece metabase.Piece
		for i, piece := range segment.Pieces {
			if i >= toKill {
				// choose a node and pieceID to shutdown
				if offlinePiece.StorageNode.IsZero() {
					offlinePiece = piece
				}
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.False(t, offlinePiece.StorageNode.IsZero())

		// shutdown node
		offlineNode := planet.FindNode(offlinePiece.StorageNode)
		require.NotNil(t, offlineNode)
		require.NoError(t, planet.StopPeer(offlineNode))

		ecRepairer := satellite.Repairer.EcRepairer

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)
		getOrderLimits, getPrivateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		ecRepairer.TestingSetMinFailures(1)
		_, piecesReport, err := ecRepairer.Get(ctx, getOrderLimits, cachedIPsAndPorts, getPrivateKey, redundancy, int64(segment.EncryptedSize))
		require.NoError(t, err)
		require.Equal(t, 1, len(piecesReport.Offline))
		require.Equal(t, 0, len(piecesReport.Failed))
		require.Equal(t, 0, len(piecesReport.Contained))
		require.Equal(t, 0, len(piecesReport.Unknown))
		require.Equal(t, int(segment.Redundancy.RequiredShares), len(piecesReport.Successful))
		require.Equal(t, offlinePiece, piecesReport.Offline[0].Piece)
	})
}

func TestECRepairerGetUnknown(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
			Satellite: testplanet.ReconfigureRS(3, 3, 6, 6),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 6, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))
		toKill := 2

		// kill nodes and track lost pieces
		var unknownPiece metabase.Piece
		for i, piece := range segment.Pieces {
			if i >= toKill {
				// choose a node to return unknown error
				if unknownPiece.StorageNode.IsZero() {
					unknownPiece = piece
				}
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.False(t, unknownPiece.StorageNode.IsZero())

		// set unknown error for download from bad node
		badNode := planet.FindNode(unknownPiece.StorageNode)
		require.NotNil(t, badNode)
		badNodeDB := badNode.DB.(*testblobs.BadDB)
		badNodeDB.SetError(errs.New("unknown error"))

		ecRepairer := satellite.Repairer.EcRepairer

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)
		getOrderLimits, getPrivateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		ecRepairer.TestingSetMinFailures(1)
		_, piecesReport, err := ecRepairer.Get(ctx, getOrderLimits, cachedIPsAndPorts, getPrivateKey, redundancy, int64(segment.EncryptedSize))
		require.NoError(t, err)
		require.Equal(t, 0, len(piecesReport.Offline))
		require.Equal(t, 0, len(piecesReport.Failed))
		require.Equal(t, 0, len(piecesReport.Contained))
		require.Equal(t, 1, len(piecesReport.Unknown))
		require.Equal(t, int(segment.Redundancy.RequiredShares), len(piecesReport.Successful))
		require.Equal(t, unknownPiece, piecesReport.Unknown[0].Piece)
	})
}

func TestECRepairerGetFailure(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			StorageNodeDB: func(index int, db storagenode.DB, log *zap.Logger) (storagenode.DB, error) {
				return testblobs.NewBadDB(log.Named("baddb"), db), nil
			},
			Satellite: testplanet.ReconfigureRS(3, 3, 6, 6),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()
		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.Equal(t, 6, len(segment.Pieces))
		require.Equal(t, 3, int(segment.Redundancy.RequiredShares))

		// calculate how many storagenodes to kill
		toKill := 2

		var onlinePieces metabase.Pieces
		for i, piece := range segment.Pieces {
			if i >= toKill {
				onlinePieces = append(onlinePieces, piece)
				continue
			}

			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.StorageNode))
			require.NoError(t, err)
		}
		require.Equal(t, 4, len(onlinePieces))

		successfulPiece := onlinePieces[0]
		offlinePiece := onlinePieces[1]
		unknownPiece := onlinePieces[2]
		corruptedPiece := onlinePieces[3]

		// stop offline node
		offlineNode := planet.FindNode(offlinePiece.StorageNode)
		require.NotNil(t, offlineNode)
		require.NoError(t, planet.StopPeer(offlineNode))

		// set unknown error for download from bad node
		badNode := planet.FindNode(unknownPiece.StorageNode)
		require.NotNil(t, badNode)
		badNodeDB := badNode.DB.(*testblobs.BadDB)
		badNodeDB.SetError(errs.New("unknown error"))

		// corrupt data for corrupted node
		corruptedNode := planet.FindNode(corruptedPiece.StorageNode)
		require.NotNil(t, corruptedNode)
		corruptedPieceID := segment.RootPieceID.Derive(corruptedPiece.StorageNode, int32(corruptedPiece.Number))
		require.NotNil(t, corruptedPieceID)
		corruptPieceData(ctx, t, planet, corruptedNode, corruptedPieceID)

		ecRepairer := satellite.Repairer.EcRepairer

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)
		getOrderLimits, getPrivateKey, cachedIPsAndPorts, err := satellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		_, piecesReport, err := ecRepairer.Get(ctx, getOrderLimits, cachedIPsAndPorts, getPrivateKey, redundancy, int64(segment.EncryptedSize))
		require.Error(t, err)
		require.Equal(t, 1, len(piecesReport.Offline))
		require.Equal(t, 1, len(piecesReport.Failed))
		require.Equal(t, 0, len(piecesReport.Contained))
		require.Equal(t, 1, len(piecesReport.Unknown))
		require.Equal(t, 1, len(piecesReport.Successful))
		require.Equal(t, offlinePiece, piecesReport.Offline[0].Piece)
		require.Equal(t, corruptedPiece, piecesReport.Failed[0].Piece)
		require.Equal(t, unknownPiece, piecesReport.Unknown[0].Piece)
		require.Equal(t, successfulPiece, piecesReport.Successful[0].Piece)
	})
}

func TestECRepairerGetDoesNameLookupIfNecessary(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		testSatellite := planet.Satellites[0]
		audits := testSatellite.Audit

		audits.Worker.Loop.Pause()
		testSatellite.RangedLoop.RangedLoop.Service.Loop.Stop()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, testSatellite, "test.bucket", "some//path", testData)
		require.NoError(t, err)

		// trigger audit
		_, err = testSatellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := testSatellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)
		require.True(t, len(segment.Pieces) > 1)

		limits, privateKey, cachedNodesInfo, err := testSatellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		for i, l := range limits {
			if l == nil {
				continue
			}
			info := cachedNodesInfo[l.Limit.StorageNodeId]
			info.LastIPPort = fmt.Sprintf("garbageXXX#:%d", i)
			cachedNodesInfo[l.Limit.StorageNodeId] = info
		}

		mock := &mockConnector{}
		ec := ecRepairerWithMockConnector(t, testSatellite, mock)

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)

		readCloser, pieces, err := ec.Get(ctx, limits, cachedNodesInfo, privateKey, redundancy, int64(segment.EncryptedSize))
		require.NoError(t, err)
		require.Len(t, pieces.Failed, 0)
		require.NotNil(t, readCloser)

		// repair will only download minimum required
		minReq := redundancy.RequiredCount()
		var numDialed int
		for _, info := range cachedNodesInfo {
			for _, dialed := range mock.addressesDialed {
				if dialed == info.LastIPPort {
					numDialed++
					if numDialed == minReq {
						break
					}
				}
			}
			if numDialed == minReq {
				break
			}
		}
		require.True(t, numDialed == minReq)
	})
}

func TestECRepairerGetPrefersCachedIPPort(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		testSatellite := planet.Satellites[0]
		audits := testSatellite.Audit

		audits.Worker.Loop.Pause()
		testSatellite.RangedLoop.RangedLoop.Service.Loop.Stop()

		ul := planet.Uplinks[0]
		testData := testrand.Bytes(8 * memory.KiB)

		err := ul.Upload(ctx, testSatellite, "test.bucket", "some//path", testData)
		require.NoError(t, err)

		// trigger audit
		_, err = testSatellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		queue := audits.VerifyQueue
		queueSegment, err := queue.Next(ctx)
		require.NoError(t, err)

		segment, err := testSatellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: queueSegment.StreamID,
			Position: queueSegment.Position,
		})
		require.NoError(t, err)
		require.True(t, len(segment.Pieces) > 1)

		limits, privateKey, cachedNodesInfo, err := testSatellite.Orders.Service.CreateGetRepairOrderLimits(ctx, segment, segment.Pieces)
		require.NoError(t, err)

		// make it so that when the cached IP is dialed, we dial the "right" address,
		// but when the "right" address is dialed (meaning it came from the OrderLimit,
		// we dial something else!
		mock := &mockConnector{
			dialInstead: make(map[string]string),
		}
		var realAddresses []string
		for i, l := range limits {
			if l == nil {
				continue
			}

			info := cachedNodesInfo[l.Limit.StorageNodeId]
			info.LastIPPort = fmt.Sprintf("garbageXXX#:%d", i)
			cachedNodesInfo[l.Limit.StorageNodeId] = info

			address := l.StorageNodeAddress.Address
			mock.dialInstead[info.LastIPPort] = address
			mock.dialInstead[address] = "utter.failure?!*"

			realAddresses = append(realAddresses, address)
		}

		ec := ecRepairerWithMockConnector(t, testSatellite, mock)

		redundancy, err := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
		require.NoError(t, err)

		readCloser, pieces, err := ec.Get(ctx, limits, cachedNodesInfo, privateKey, redundancy, int64(segment.EncryptedSize))
		require.NoError(t, err)
		require.Len(t, pieces.Failed, 0)
		require.NotNil(t, readCloser)
		// repair will only download minimum required.
		minReq := redundancy.RequiredCount()
		var numDialed int
		for _, info := range cachedNodesInfo {
			for _, dialed := range mock.addressesDialed {
				if dialed == info.LastIPPort {
					numDialed++
					if numDialed == minReq {
						break
					}
				}
			}
			if numDialed == minReq {
				break
			}
		}
		require.True(t, numDialed == minReq)
		// and that the right address was never dialed directly
		require.NotContains(t, mock.addressesDialed, realAddresses)
	})
}

func TestSegmentInExcludedCountriesRepair(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 20,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = true
					config.Repairer.MaxExcessRateOptimalThreshold = 0.0
				},
				testplanet.ReconfigureRS(3, 5, 8, 10),
				testplanet.RepairExcludedCountryCodes([]string{"FR", "BE"}),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		remotePieces := segment.Pieces
		require.GreaterOrEqual(t, len(segment.Pieces), int(segment.Redundancy.OptimalShares))

		numExcluded := 5
		var nodesInExcluded storj.NodeIDList
		for i := 0; i < numExcluded; i++ {
			err = planet.Satellites[0].Overlay.Service.TestNodeCountryCode(ctx, remotePieces[i].StorageNode, "FR")
			require.NoError(t, err)
			nodesInExcluded = append(nodesInExcluded, remotePieces[i].StorageNode)
		}

		// make extra pieces after the optimal threshold bad, so we know there are exactly
		// OptimalShares retrievable shares. numExcluded of them are in an excluded country.
		for i := int(segment.Redundancy.OptimalShares); i < len(remotePieces); i++ {
			err = planet.StopNodeAndUpdate(ctx, planet.FindNode(remotePieces[i].StorageNode))
			require.NoError(t, err)
		}

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the segment was removed from the repair queue
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)

		// Verify the segment has been repaired
		segmentAfterRepair, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.NotEqual(t, segment.Pieces, segmentAfterRepair.Pieces)
		require.GreaterOrEqual(t, len(segmentAfterRepair.Pieces), int(segmentAfterRepair.Redundancy.OptimalShares))

		// the number of nodes that should still be online holding intact pieces, not in
		// excluded countries
		expectHealthyNodes := int(segment.Redundancy.OptimalShares) - numExcluded
		// repair should create this many new pieces to get the segment up to OptimalShares
		// shares, not counting excluded-country nodes
		expectNewPieces := int(segment.Redundancy.OptimalShares) - expectHealthyNodes
		// so there should be this many pieces after repair, not counting excluded-country
		// nodes
		expectPiecesAfterRepair := expectHealthyNodes + expectNewPieces
		// so there should be this many excluded-country pieces left in the segment (we
		// couldn't keep all of them, or we would have had more than TotalShares pieces).
		expectRemainingExcluded := int(segment.Redundancy.TotalShares) - expectPiecesAfterRepair

		// check excluded area nodes are no longer being used
		var found int
		for _, nodeID := range nodesInExcluded {
			for _, p := range segmentAfterRepair.Pieces {
				if p.StorageNode == nodeID {
					found++
					break
				}
			}
		}
		require.Equal(t, found, expectRemainingExcluded, "found wrong number of excluded-country pieces after repair")
		nodesInPointer := make(map[storj.NodeID]bool)
		for _, n := range segmentAfterRepair.Pieces {
			// check for duplicates
			_, ok := nodesInPointer[n.StorageNode]
			require.False(t, ok)
			nodesInPointer[n.StorageNode] = true
		}
	})
}

// - 7 storage nodes
// - pieces uploaded to 4 or 5 nodes
// - mark one node holding a piece in excluded area
// - put one other node holding a piece offline
// - run the checker and check the segment is in the repair queue
// - run the repairer
// - check the segment has been repaired and that:
//   - piece in excluded is still there
//   - piece held by offline node is not
//   - there are no duplicate
func TestSegmentInExcludedCountriesRepairIrreparable(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 7,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Repairer.InMemoryRepair = true
				},
				testplanet.ReconfigureRS(2, 3, 4, 5),
				testplanet.RepairExcludedCountryCodes([]string{"FR", "BE"}),
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")

		remotePieces := segment.Pieces
		require.GreaterOrEqual(t, len(remotePieces), int(segment.Redundancy.OptimalShares))

		err = planet.Satellites[0].Overlay.Service.TestNodeCountryCode(ctx, remotePieces[1].StorageNode, "FR")
		require.NoError(t, err)
		nodeInExcluded := remotePieces[0].StorageNode
		offlineNode := remotePieces[2].StorageNode
		// make  one unhealthy
		err = planet.StopNodeAndUpdate(ctx, planet.FindNode(offlineNode))
		require.NoError(t, err)

		// trigger checker with ranged loop to add segment to repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		count, err := satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		satellite.Repair.Repairer.Loop.Restart()
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.Loop.Pause()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// Verify that the segment was removed
		count, err = satellite.DB.RepairQueue().Count(ctx)
		require.NoError(t, err)
		require.Zero(t, count)

		// Verify the segment has been repaired
		segmentAfterRepair, _ := getRemoteSegment(ctx, t, satellite, planet.Uplinks[0].Projects[0].ID, "testbucket")
		require.NotEqual(t, segment.Pieces, segmentAfterRepair.Pieces)
		require.GreaterOrEqual(t, len(segmentAfterRepair.Pieces), int(segment.Redundancy.OptimalShares))

		// check node in excluded area still exists
		var nodeInExcludedAreaFound bool
		var offlineNodeFound bool
		for _, p := range segmentAfterRepair.Pieces {
			if p.StorageNode == nodeInExcluded {
				nodeInExcludedAreaFound = true
			}
			if p.StorageNode == offlineNode {
				offlineNodeFound = true
			}
		}
		require.True(t, nodeInExcludedAreaFound, fmt.Sprintf("node %s not in segment, but should be\n", nodeInExcluded.String()))
		require.False(t, offlineNodeFound, fmt.Sprintf("node %s in segment, but should not be\n", offlineNode.String()))

		nodesInPointer := make(map[storj.NodeID]bool)
		for _, n := range segmentAfterRepair.Pieces {
			// check for duplicates
			_, ok := nodesInPointer[n.StorageNode]
			require.False(t, ok)
			nodesInPointer[n.StorageNode] = true
		}
	})
}

func reputationRatio(info reputation.Info) float64 {
	return info.AuditReputationAlpha / (info.AuditReputationAlpha + info.AuditReputationBeta)
}

func TestRepairClumpedPieces(t *testing.T) {
	// Test that if nodes change IPs such that multiple pieces of a segment
	// reside in the same network, that segment will be considered unhealthy
	// by the repair checker and it will be repaired by the repair worker.
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 3, 4, 4),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Checker.DoDeclumping = true
					config.Repairer.DoDeclumping = true
				},
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")
		remotePiecesBefore := segment.Pieces

		// that segment should be ignored by repair checker for now
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		injuredSegment, err := satellite.DB.RepairQueue().Select(ctx, nil, nil)
		require.Error(t, err)
		if !queue.ErrEmpty.Has(err) {
			require.FailNow(t, "Should get ErrEmptyQueue, but got", err)
		}
		require.Nil(t, injuredSegment)

		// pieces list has not changed
		segment, _ = getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")
		remotePiecesAfter := segment.Pieces
		require.Equal(t, remotePiecesBefore, remotePiecesAfter)

		// now move the network of one storage node holding a piece, so that it's the same as another
		node0 := planet.FindNode(remotePiecesAfter[0].StorageNode)
		node1 := planet.FindNode(remotePiecesAfter[1].StorageNode)

		local := node0.Contact.Service.Local()
		checkInInfo := overlay.NodeCheckInInfo{
			NodeID:     node0.ID(),
			Address:    &pb.NodeAddress{Address: local.Address},
			LastIPPort: local.Address,
			LastNet:    node1.Contact.Service.Local().Address,
			IsUp:       true,
			Operator:   &local.Operator,
			Capacity:   &local.Capacity,
			Version:    &local.Version,
		}

		require.NoError(t, satellite.DB.OverlayCache().UpdateCheckIn(ctx, checkInInfo, time.Now().UTC(), overlay.NodeSelectionConfig{}))

		require.NoError(t, satellite.RangedLoop.Repair.Observer.RefreshReliabilityCache(ctx))

		// running repair checker again should put the segment into the repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// and subsequently running the repair worker should pull that off the queue and repair it
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// confirm that the segment now has exactly one piece on (node0 or node1)
		// and still has the right number of pieces.
		segment, _ = getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")
		require.Len(t, segment.Pieces, 4)
		foundOnFirstNetwork := 0
		for _, piece := range segment.Pieces {
			if piece.StorageNode.Compare(node0.ID()) == 0 || piece.StorageNode.Compare(node1.ID()) == 0 {
				foundOnFirstNetwork++
			}
		}
		require.Equalf(t, 1, foundOnFirstNetwork,
			"%v should only include one of %s or %s", segment.Pieces, node0.ID(), node1.ID())
	})
}

func TestRepairClumpedPiecesBasedOnTags(t *testing.T) {
	signer := testidentity.MustPregeneratedIdentity(50, storj.LatestIDVersion())
	tempDir := t.TempDir()
	pc := identity.PeerConfig{
		CertPath: filepath.Join(tempDir, "identity.cert"),
	}
	require.NoError(t, pc.Save(signer.PeerIdentity()))

	placementConfig := fmt.Sprintf(`
placements:
- id: 0
  name: default
  invariant: maxcontrol("tag:%s/datacenter", 2)`, signer.ID.String())

	placementConfigPath := filepath.Join(tempDir, "placement.yaml")
	require.NoError(t, os.WriteFile(placementConfigPath, []byte(placementConfig), 0755))

	// Test that if nodes change IPs such that multiple pieces of a segment
	// reside in the same network, that segment will be considered unhealthy
	// by the repair checker and it will be repaired by the repair worker.
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 6,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.Combine(
				testplanet.ReconfigureRS(2, 3, 4, 4),
				func(log *zap.Logger, index int, config *satellite.Config) {
					config.Checker.DoDeclumping = true
					config.Repairer.DoDeclumping = true
					config.Placement.PlacementRules = placementConfigPath
					config.TagAuthorities = pc.CertPath
				},
			),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplinkPeer := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Worker.Loop.Pause()

		satellite.RangedLoop.RangedLoop.Service.Loop.Stop()
		satellite.Repair.Repairer.Loop.Pause()

		var testData = testrand.Bytes(8 * memory.KiB)
		// first, upload some remote data
		err := uplinkPeer.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		segment, _ := getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")
		remotePiecesBefore := segment.Pieces

		// that segment should be ignored by repair checker for now
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		injuredSegment, err := satellite.DB.RepairQueue().Select(ctx, nil, nil)
		require.Error(t, err)
		if !queue.ErrEmpty.Has(err) {
			require.FailNow(t, "Should get ErrEmptyQueue, but got", err)
		}
		require.Nil(t, injuredSegment)

		// pieces list has not changed
		segment, _ = getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")
		remotePiecesAfter := segment.Pieces
		require.Equal(t, remotePiecesBefore, remotePiecesAfter)

		// now move the network of one storage node holding a piece, so that it's the same as another
		require.NoError(t, satellite.DB.OverlayCache().UpdateNodeTags(ctx, []nodeselection.NodeTag{
			{
				NodeID:   planet.FindNode(remotePiecesAfter[0].StorageNode).ID(),
				SignedAt: time.Now(),
				Name:     "datacenter",
				Value:    []byte("dc1"),
				Signer:   signer.ID,
			},
			{
				NodeID:   planet.FindNode(remotePiecesAfter[1].StorageNode).ID(),
				SignedAt: time.Now(),
				Name:     "datacenter",
				Value:    []byte("dc1"),
				Signer:   signer.ID,
			},
			{
				NodeID:   planet.FindNode(remotePiecesAfter[2].StorageNode).ID(),
				SignedAt: time.Now(),
				Name:     "datacenter",
				Value:    []byte("dc1"),
				Signer:   signer.ID,
			},
		}))

		require.NoError(t, satellite.RangedLoop.Repair.Observer.RefreshReliabilityCache(ctx))

		// running repair checker again should put the segment into the repair queue
		_, err = satellite.RangedLoop.RangedLoop.Service.RunOnce(ctx)
		require.NoError(t, err)

		// and subsequently running the repair worker should pull that off the queue and repair it
		satellite.Repair.Repairer.Loop.TriggerWait()
		satellite.Repair.Repairer.WaitForPendingRepairs()

		// confirm that the segment now has exactly one piece on (node0 or node1)
		// and still has the right number of pieces.
		segment, _ = getRemoteSegment(ctx, t, satellite, uplinkPeer.Projects[0].ID, "testbucket")
		require.Len(t, segment.Pieces, 4)
		foundOnDC1 := 0
		for _, piece := range segment.Pieces {
			for i := 0; i < 3; i++ {
				if piece.StorageNode.Compare(remotePiecesAfter[i].StorageNode) == 0 {
					foundOnDC1++
				}
			}
		}
		require.Equalf(t, 2, foundOnDC1,
			"%v should be moved out from at least one node", segment.Pieces)
	})
}
