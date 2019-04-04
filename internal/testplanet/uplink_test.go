// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

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
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Skip("flaky")

		// first, upload some remote data
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		testData := make([]byte, 1*memory.MiB)
		_, err := rand.Read(testData)
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

				err = satellite.Overlay.Service.Delete(ctx, node.ID())
				require.NoError(t, err)
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := ul.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, testData, newData)
	})
}

func TestUploadDownloadOneUplinksInParallel(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	dataToUpload := make([][]byte, 5)
	for i := 0; i < len(dataToUpload); i++ {
		dataToUpload[i] = make([]byte, 100*memory.KiB.Int()+(i*100*memory.KiB.Int()))
		_, err := rand.Read(dataToUpload[i])
		require.NoError(t, err)
	}

	var group errgroup.Group
	for i, data := range dataToUpload {
		group.Go(func() error {
			index := strconv.Itoa(i)
			err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket"+index, "test/path"+index, data)
			return err
		})
	}
	err = group.Wait()
	require.NoError(t, err)

	for i, expectedData := range dataToUpload {
		group.Go(func() error {
			index := strconv.Itoa(i)
			data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket"+index, "test/path"+index)
			require.Equal(t, expectedData, data)
			return err
		})
	}
	err = group.Wait()
	require.NoError(t, err)
}

func TestUploadDownloadMultipleUplinksInParallel(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	numberOfUplinks := 5
	planet, err := testplanet.New(t, 1, 6, numberOfUplinks)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	dataToUpload := make([][]byte, numberOfUplinks)
	for i := 0; i < len(dataToUpload); i++ {
		dataToUpload[i] = make([]byte, 100*memory.KiB.Int()+(i*100*memory.KiB.Int()))
		_, err := rand.Read(dataToUpload[i])
		require.NoError(t, err)
	}

	var group errgroup.Group
	for i, data := range dataToUpload {
		group.Go(func() error {
			index := strconv.Itoa(i)
			err = planet.Uplinks[i].Upload(ctx, planet.Satellites[0], "testbucket"+index, "test/path"+index, data)
			return err
		})
	}
	err = group.Wait()
	require.NoError(t, err)

	for i, expectedData := range dataToUpload {
		group.Go(func() error {
			index := strconv.Itoa(i)
			data, err := planet.Uplinks[i].Download(ctx, planet.Satellites[0], "testbucket"+index, "test/path"+index)
			require.Equal(t, expectedData, data)
			return err
		})
	}
	err = group.Wait()
	require.NoError(t, err)
}
