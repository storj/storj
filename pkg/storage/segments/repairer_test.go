// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

func TestSegmentStoreRepair(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.Repair.Checker.Loop.Stop()
		// stop discovery service so that we do not get a race condition when we delete nodes from overlay cache
		satellite.Discovery.Service.Discovery.Stop()
		satellite.Discovery.Service.Refresh.Stop()

		testData := testrand.Bytes(1 * memory.MiB)

		err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     4,
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
		toKill := numPieces - (int(minReq) + 1)
		// we should have enough storage nodes to repair on
		assert.True(t, (numStorageNodes-toKill) >= numPieces)

		// kill nodes and track lost pieces
		var lostPieces []int32
		nodesToKill := make(map[storj.NodeID]bool)
		nodesToKeepAlive := make(map[storj.NodeID]bool)
		for i, piece := range remotePieces {
			if i >= toKill {
				nodesToKeepAlive[piece.NodeId] = true
				continue
			}
			nodesToKill[piece.NodeId] = true
			lostPieces = append(lostPieces, piece.GetPieceNum())
		}
		for _, node := range planet.StorageNodes {
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
		assert.NotNil(t, repairer)

		err = repairer.Repair(ctx, path)
		assert.NoError(t, err)

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
		assert.NoError(t, err)
		assert.Equal(t, newData, testData)

		// updated pointer should not contain any of the killed nodes
		pointer, err = metainfo.Get(ctx, path)
		assert.NoError(t, err)

		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			assert.False(t, nodesToKill[piece.NodeId])
		}
	})
}
