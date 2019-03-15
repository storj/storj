// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
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

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test/bucket", "test/path", expectedData)
	assert.NoError(t, err)

	for _, tt := range []struct {
		offset, size int64
	}{
		{0, 1510},
		{1513, 1584},
		{13581, 4783},
	} {
		download, err := planet.Uplinks[0].DownloadStream(ctx, planet.Satellites[0], "test/bucket", "test/path")
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
}
