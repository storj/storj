// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/uplink"
)

func TestSegmentStoreRepair(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 12, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.Repair.Checker.Loop.Stop()
		// stop discovery service so that we do not get a race condition when we delete nodes from overlay cache
		satellite.Discovery.Service.Discovery.Stop()
		satellite.Discovery.Service.Refresh.Stop()

		testData := make([]byte, 1*memory.MiB)
		_, err := rand.Read(testData)
		require.NoError(t, err)

		err = ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     3,
			RepairThreshold:  5,
			SuccessThreshold: 7,
			MaxThreshold:     7,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from metainfo
		metainfo := satellite.Metainfo.Service
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

		// calculate how many storagenodes to kill
		numStorageNodes := len(planet.StorageNodes)
		redundancy := pointer.GetRemote().GetRedundancy()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		minReq := redundancy.GetMinReq()
		numPieces := len(remotePieces)
		// disqualify one storage node
		toDisqualify := 1
		toKill := numPieces - (int(minReq+1) + toDisqualify)
		require.True(t, toKill >= 1)
		// we should have enough storage nodes to repair on
		require.True(t, (numStorageNodes-toKill-toDisqualify) >= numPieces)

		// kill nodes and track lost pieces
		var lostPieces []int32
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
			lostPieces = append(lostPieces, piece.GetPieceNum())
		}
		for _, node := range planet.StorageNodes {
			if nodesToDisqualify[node.ID()] {
				disqualifyNode(t, ctx, satellite, node.ID())
				continue
			}
			if nodesToKill[node.ID()] {
				err = planet.StopPeer(node)
				require.NoError(t, err)
				_, err = satellite.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
			}
		}

		// repair segment
		os := satellite.Orders.Service
		oc := satellite.Overlay.Service
		ec := ecclient.NewClient(satellite.Transport, 0)
		repairer := segments.NewSegmentRepairer(metainfo, os, oc, ec, satellite.Identity, time.Minute)
		require.NotNil(t, repairer)

		err = repairer.Repair(ctx, path)
		require.NoError(t, err)

		// kill one of the nodes kept alive to ensure repair worked
		for _, node := range planet.StorageNodes {
			if nodesToKeepAlive[node.ID()] {
				err = planet.StopPeer(node)
				require.NoError(t, err)
				_, err = satellite.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
				break
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := ul.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, newData, testData)

		// updated pointer should not contain any of the killed nodes
		pointer, err = metainfo.Get(ctx, path)
		require.NoError(t, err)

		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			require.False(t, nodesToKill[piece.NodeId])
			require.False(t, nodesToDisqualify[piece.NodeId])
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
