// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
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
	var repairMaxExcessRateOptimalThreshold float64

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 14,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Overlay.Node.OnlineWindow = 0
				repairMaxExcessRateOptimalThreshold = config.Repairer.MaxExcessRateOptimalThreshold
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplinkPeer := planet.Uplinks[0]
		satellitePeer := planet.Satellites[0]
		// stop discovery service so that we do not get a race condition when we delete nodes from overlay cache
		satellitePeer.Discovery.Service.Discovery.Stop()
		satellitePeer.Discovery.Service.Refresh.Stop()
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellitePeer.Audit.Service.Loop.Stop()

		satellitePeer.Repair.Checker.Loop.Pause()
		satellitePeer.Repair.Repairer.Loop.Pause()

		var (
			testData         = testrand.Bytes(8 * memory.KiB)
			minThreshold     = 3
			successThreshold = 7
		)
		err := uplinkPeer.UploadWithConfig(ctx, satellitePeer, &uplink.RSConfig{
			MinThreshold:     minThreshold,
			RepairThreshold:  5,
			SuccessThreshold: successThreshold,
			MaxThreshold:     10,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		pointer, path := getRemoteSegment(t, ctx, satellitePeer)

		// calculate how many storagenodes to kill
		redundancy := pointer.GetRemote().GetRedundancy()
		minReq := redundancy.GetMinReq()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		numPieces := len(remotePieces)
		// disqualify one storage node
		toDisqualify := 1
		toKill := numPieces - toDisqualify - int(minReq+1)
		require.True(t, toKill >= 1)
		maxNumRepairedPieces := int(
			math.Ceil(
				float64(successThreshold) * (1 + repairMaxExcessRateOptimalThreshold),
			),
		)
		numStorageNodes := len(planet.StorageNodes)
		// Ensure that there are enough storage nodes to upload repaired segments
		require.Falsef(t,
			(numStorageNodes-toKill-toDisqualify) < maxNumRepairedPieces,
			"there is not enough available nodes for repairing: need= %d, have= %d",
			maxNumRepairedPieces, (numStorageNodes - toKill - toDisqualify),
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
				disqualifyNode(t, ctx, satellitePeer, node.ID())
				continue
			}
			if nodesToKill[node.ID()] {
				err = planet.StopPeer(node)
				require.NoError(t, err)
				_, err = satellitePeer.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
			}
		}

		satellitePeer.Repair.Checker.Loop.Restart()
		satellitePeer.Repair.Checker.Loop.TriggerWait()
		satellitePeer.Repair.Checker.Loop.Pause()
		satellitePeer.Repair.Repairer.Loop.Restart()
		satellitePeer.Repair.Repairer.Loop.TriggerWait()
		satellitePeer.Repair.Repairer.Loop.Pause()
		satellitePeer.Repair.Repairer.Limiter.Wait()

		// repaired segment should not contain any piece in the killed and DQ nodes
		metainfoService := satellitePeer.Metainfo.Service
		pointer, err = metainfoService.Get(ctx, path)
		require.NoError(t, err)

		nodesToKillForMinThreshold := len(remotePieces) - minThreshold
		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			require.NotContains(t, nodesToKill, piece.NodeId, "there shouldn't be pieces in killed nodes")
			require.NotContains(t, nodesToDisqualify, piece.NodeId, "there shouldn't be pieces in DQ nodes")

			// Kill the original nodes which were kept alive to ensure that we can
			// download from the new nodes that the repaired pieces have been uploaded
			if _, ok := nodesToKeepAlive[piece.NodeId]; ok && nodesToKillForMinThreshold > 0 {
				stopNodeByID(t, ctx, planet, piece.NodeId)
				nodesToKillForMinThreshold--
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellitePeer, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)
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
		satellitePeer := planet.Satellites[0]
		// stop discovery service so that we do not get a race condition when we delete nodes from overlay cache
		satellitePeer.Discovery.Service.Discovery.Stop()
		satellitePeer.Discovery.Service.Refresh.Stop()

		satellitePeer.Repair.Checker.Loop.Pause()
		satellitePeer.Repair.Repairer.Loop.Pause()

		testData := testrand.Bytes(8 * memory.KiB)

		err := uplinkPeer.UploadWithConfig(ctx, satellitePeer, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  5,
			SuccessThreshold: 7,
			MaxThreshold:     7,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from metainfo
		metainfo := satellitePeer.Metainfo.Service
		listResponse, _, err := metainfo.List(ctx, "", "", "", true, 0, 0)
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
		var lostPieces []int32
		nodesToDisqualify := make(map[storj.NodeID]bool)
		nodesToKeepAlive := make(map[storj.NodeID]bool)

		for i, piece := range remotePieces {
			if i >= toDisqualify {
				nodesToKeepAlive[piece.NodeId] = true
				continue
			}
			nodesToDisqualify[piece.NodeId] = true
			lostPieces = append(lostPieces, piece.GetPieceNum())
		}

		for _, node := range planet.StorageNodes {
			if nodesToDisqualify[node.ID()] {
				disqualifyNode(t, ctx, satellitePeer, node.ID())
			}
		}

		err = satellitePeer.Repair.Checker.RefreshReliabilityCache(ctx)
		require.NoError(t, err)

		satellitePeer.Repair.Checker.Loop.TriggerWait()
		satellitePeer.Repair.Repairer.Loop.TriggerWait()
		satellitePeer.Repair.Repairer.Limiter.Wait()

		// kill nodes kept alive to ensure repair worked
		for _, node := range planet.StorageNodes {
			if nodesToKeepAlive[node.ID()] {
				err = planet.StopPeer(node)
				require.NoError(t, err)

				_, err = satellitePeer.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := uplinkPeer.Download(ctx, satellitePeer, "testbucket", "test/path")
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

// TestDataRepairUploadLimits does the following:
// - Uploads test data to nodes
// - Get one segment of that data to check in which nodes its pieces are stored
// - Kills as many nodes as needed which store such segment pieces
// - Triggers data repair
// - Verify that the number of pieces which repaired has uploaded don't overpass
//	 the established limit (success threshold + % of excess)
func TestDataRepairUploadLimit(t *testing.T) {
	var repairMaxExcessRateOptimalThreshold float64

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 13,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				repairMaxExcessRateOptimalThreshold = config.Repairer.MaxExcessRateOptimalThreshold
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		// stop discovery service so that we do not get a race condition when we delete nodes from overlay cache
		satellite.Discovery.Service.Discovery.Stop()
		satellite.Discovery.Service.Refresh.Stop()
		// stop audit to prevent possible interactions i.e. repair timeout problems
		satellite.Audit.Service.Loop.Stop()
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
					float64(successThreshold) * (1 + repairMaxExcessRateOptimalThreshold),
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
					err = planet.StopPeer(node)
					require.NoError(t, err)
					_, err = satellite.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
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

func isDisqualified(t *testing.T, ctx *testcontext.Context, satellite *satellite.Peer, nodeID storj.NodeID) bool {
	node, err := satellite.Overlay.Service.Get(ctx, nodeID)
	require.NoError(t, err)

	return node.Disqualified != nil
}

func disqualifyNode(t *testing.T, ctx *testcontext.Context, satellite *satellite.Peer, nodeID storj.NodeID) {
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
	t *testing.T, ctx context.Context, satellite *satellite.Peer,
) (_ *pb.Pointer, path string) {
	t.Helper()

	// get a remote segment from metainfo
	metainfo := satellite.Metainfo.Service
	listResponse, _, err := metainfo.List(ctx, "", "", "", true, 0, 0)
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

			for _, sat := range planet.Satellites {
				_, err = sat.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
			}

			break
		}
	}
}
