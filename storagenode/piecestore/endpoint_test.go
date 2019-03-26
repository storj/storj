// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/uplink/piecestore"
)

func TestUploadAndPartialDownload(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	expectedData := make([]byte, 100*memory.KiB)
	_, err = rand.Read(expectedData)
	require.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
	assert.NoError(t, err)

	var totalDownload int64
	for _, tt := range []struct {
		offset, size int64
	}{
		{0, 1510},
		{1513, 1584},
		{13581, 4783},
	} {
		if piecestore.DefaultConfig.InitialStep < tt.size {
			t.Fatal("test expects initial step to be larger than size to download")
		}
		totalDownload += piecestore.DefaultConfig.InitialStep

		download, err := planet.Uplinks[0].DownloadStream(ctx, planet.Satellites[0], "testbucket", "test/path")
		require.NoError(t, err)

		pos, err := download.Seek(tt.offset, io.SeekStart)
		require.NoError(t, err)
		assert.Equal(t, pos, tt.offset)

		data := make([]byte, tt.size)
		n, err := io.ReadFull(download, data)
		require.NoError(t, err)
		assert.Equal(t, int(tt.size), n)

		assert.Equal(t, expectedData[tt.offset:tt.offset+tt.size], data)

		require.NoError(t, download.Close())
	}

	var totalBandwidthUsage bandwidth.Usage
	for _, storagenode := range planet.StorageNodes {
		usage, err := storagenode.DB.Bandwidth().Summary(ctx, time.Now().Add(-10*time.Hour), time.Now().Add(10*time.Hour))
		require.NoError(t, err)
		totalBandwidthUsage.Add(usage)
	}

	err = planet.Uplinks[0].Delete(ctx, planet.Satellites[0], "testbucket", "test/path")
	require.NoError(t, err)
	_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
	require.Error(t, err)

	// check rough limits for the upload and download
	totalUpload := int64(len(expectedData))
	t.Log(totalUpload, totalBandwidthUsage.Put, int64(len(planet.StorageNodes))*totalUpload)
	assert.True(t, totalUpload < totalBandwidthUsage.Put && totalBandwidthUsage.Put < int64(len(planet.StorageNodes))*totalUpload)
	t.Log(totalDownload, totalBandwidthUsage.Get, int64(len(planet.StorageNodes))*totalDownload)
	assert.True(t, totalBandwidthUsage.Get < int64(len(planet.StorageNodes))*totalDownload)
}
