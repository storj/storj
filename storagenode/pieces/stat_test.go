// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
)

func TestMonitoredBlobWriter(t *testing.T) {

	dir, err := filestore.NewDir(zaptest.NewLogger(t), t.TempDir())
	require.NoError(t, err)

	blobs := filestore.New(zaptest.NewLogger(t), dir, filestore.Config{})
	defer func() { require.NoError(t, blobs.Close()) }()

	ctx := testcontext.New(t)
	f1, err := blobs.Create(ctx, blobstore.BlobRef{
		Namespace: []byte("test"),
		Key:       []byte("key"),
	})
	require.NoError(t, err)

	statName := "test_monitored_blob_writer"
	m1 := MonitorBlobWriter(statName, f1)

	_, err = m1.Write([]byte("01234"))
	require.NoError(t, err)
	_, err = m1.Write([]byte("56789"))
	require.NoError(t, err)
	err = m1.Commit(ctx)
	require.NoError(t, err)

	found := false
	mon.Stats(func(key monkit.SeriesKey, field string, val float64) {
		if key.Measurement == statName && field == "count" {
			found = true
			require.Equal(t, float64(1), val)
			require.Equal(t, "small", key.Tags.Get("size"))
		}
	})
	require.True(t, found)

}

func TestMonitoredHash(t *testing.T) {

	statName := "test_monitored_hash"
	m1 := MonitorHash(statName, sha256.New())

	_, err := m1.Write([]byte("01234"))
	require.NoError(t, err)
	_, err = m1.Write([]byte("56789"))
	require.NoError(t, err)
	hash := m1.Sum([]byte{})
	require.NoError(t, err)
	require.Equal(t, "84d89877f0d4041efb6bf91a16f0248f2fd573e6af05c19f96bedb9f882f7882", hex.EncodeToString(hash))

	found := false
	mon.Stats(func(key monkit.SeriesKey, field string, val float64) {
		if key.Measurement == statName && field == "count" {
			found = true
			require.Equal(t, float64(1), val)
			require.Equal(t, "small", key.Tags.Get("size"))
		}
	})
	require.True(t, found)

}
