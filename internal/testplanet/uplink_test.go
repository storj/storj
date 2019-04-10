// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

func TestUploadDownload(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	expectedData := make([]byte, 1*memory.MiB)
	_, err = rand.Read(expectedData)
	assert.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
	assert.NoError(t, err)

	data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
	assert.NoError(t, err)

	assert.Equal(t, expectedData, data)
}

func TestDownloadWithSomeNodesOffline(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 3)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	// first, upload some remote data
	ul := planet.Uplinks[0]
	satellite := planet.Satellites[0]

	// stop discovery service so that we do not get a race condition when we delete nodes from overlay cache
	satellite.Discovery.Service.Discovery.Stop()

	testData := make([]byte, 1*memory.MiB)
	_, err = rand.Read(testData)
	require.NoError(t, err)

	err = ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
		MinThreshold:     2,
		RepairThreshold:  3,
		SuccessThreshold: 4,
		MaxThreshold:     5,
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
		require.NoError(t, err)
		if pointer.GetType() == pb.Pointer_REMOTE {
			break
		}
	}

	// calculate how many storagenodes to kill
	redundancy := pointer.GetRemote().GetRedundancy()
	remotePieces := pointer.GetRemote().GetRemotePieces()
	minReq := redundancy.GetMinReq()
	numPieces := len(remotePieces)
	toKill := numPieces - int(minReq)

	nodesToKill := make(map[storj.NodeID]bool)
	for i, piece := range remotePieces {
		if i >= toKill {
			continue
		}
		nodesToKill[piece.NodeId] = true
	}

	for _, node := range planet.StorageNodes {
		if nodesToKill[node.ID()] {
			err = planet.StopPeer(node)
			require.NoError(t, err)

			// mark node as offline in overlay cache
			_, err = satellite.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
			require.NoError(t, err)
		}
	}

	// we should be able to download data without any of the original nodes
	newData, err := ul.Download(ctx, satellite, "testbucket", "test/path")
	require.NoError(t, err)
	require.Equal(t, testData, newData)
}
