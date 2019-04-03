// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

func TestSegmentStoreRepair(t *testing.T) {
	t.Skip("flaky")

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		satellite.Repair.Checker.Loop.Stop()

		testData := make([]byte, 1*memory.MiB)
		_, err := rand.Read(testData)
		assert.NoError(t, err)

		err = ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     1,
			RepairThreshold:  2,
			SuccessThreshold: 3,
			MaxThreshold:     4,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := satellite.Metainfo.Service
		listResponse, _, err := pdb.List("", "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = pdb.Get(path)
			assert.NoError(t, err)
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
		toKill := numPieces - int(minReq)
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
				assert.NoError(t, err)
			}
		}

		// repair segment
		os := satellite.Orders.Service
		oc := satellite.Overlay.Service
		ec := ecclient.NewClient(satellite.Transport, 0)
		repairer := segments.NewSegmentRepairer(pdb, os, oc, ec, satellite.Identity, time.Minute)
		assert.NotNil(t, repairer)

		err = repairer.Repair(ctx, path, lostPieces)
		assert.NoError(t, err)

		// kill nodes kept alive to ensure repair worked
		for _, node := range planet.StorageNodes {
			if nodesToKeepAlive[node.ID()] {
				err = planet.StopPeer(node)
				assert.NoError(t, err)
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := ul.Download(ctx, satellite, "testbucket", "test/path")
		assert.NoError(t, err)
		assert.Equal(t, newData, testData)

		// updated pointer should not contain any of the killed nodes
		pointer, err = pdb.Get(path)
		assert.NoError(t, err)

		remotePieces = pointer.GetRemote().GetRemotePieces()
		for _, piece := range remotePieces {
			assert.False(t, nodesToKill[piece.NodeId])
		}
	})
}
