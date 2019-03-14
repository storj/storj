// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storj"
)

func TestSegmentStoreRepair(t *testing.T) {
	numStorageNodes := 10
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: numStorageNodes, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		uplink := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		testData := make([]byte, 5*memory.MiB)
		_, err := rand.Read(testData)
		assert.NoError(t, err)

		err = uplink.Upload(ctx, satellite, "test/bucket", "test/path", testData)
		assert.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := satellite.Metainfo.Service
		listResponse, _, err := pdb.List("", "", "", true, 0, 0)
		assert.NoError(t, err)

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
		redundancy := pointer.GetRemote().GetRedundancy()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		minReq := redundancy.GetRepairThreshold()
		numPieces := len(remotePieces)
		toKill := numPieces - int(minReq)
		// we should have enough storage nodes to repair on
		assert.True(t, (numStorageNodes-toKill) >= numPieces)

		// kill nodes and track lost pieces
		var lostPieces []int32
		nodesToKill := make(map[storj.NodeID]bool)
		for i, piece := range remotePieces {
			if i >= toKill {
				break
			}
			nodesToKill[piece.NodeId] = true
			lostPieces = append(lostPieces, piece.GetPieceNum())
		}
		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				planet.StopPeer(node)
			}
		}

		// repair segment
		overlayDB := satellite.DB.OverlayCache()
		statDB := satellite.DB.StatDB()
		oc := overlay.NewCache(overlayDB, statDB)
		as := satellite.Metainfo.Allocation
		ec := ecclient.NewClient(uplink.Transport, 0)
		repairer := segments.NewSegmentRepairer(pdb, as, oc, ec, satellite.Identity, &overlay.NodeSelectionConfig{})
		assert.NotNil(t, repairer)

		err = repairer.Repair(ctx, path, lostPieces)
		assert.NoError(t, err)

		// updated pointer should not contain any of the killed nodes
		pointer, err = pdb.Get(path)
		assert.NoError(t, err)

		remotePieces = pointer.GetRemote().GetRemotePieces()
		assert.Equal(t, numPieces, len(remotePieces))
		for _, piece := range remotePieces {
			assert.False(t, nodesToKill[piece.NodeId])
		}
	})
}
