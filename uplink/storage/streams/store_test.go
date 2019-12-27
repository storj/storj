// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/uplink/ecclient"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/storage/meta"
	"storj.io/storj/uplink/storage/segments"
	"storj.io/storj/uplink/storage/streams"
)

const (
	TestEncKey = "test-encryption-key"
)

// TestStreamsInterruptedDelete tests a special case where the delete command is
// interrupted before all segments are deleted. On subsequent calls to
// streamStore.Delete we want to ensure it completes the delete without error,
// even though some segments have already been deleted.
func TestStreamsInterruptedDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		metainfoClient, segmentStore, streamStore := storeTestSetup(t, ctx, planet, memory.KiB.Int64())
		defer ctx.Check(metainfoClient.Close)

		bucketName := "bucket-name"
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		content := testrand.Bytes(2 * memory.KiB)
		path := "mypath"
		fullPath := storj.JoinPaths(bucketName, "mypath")

		_, err = streamStore.Put(ctx, fullPath, storj.EncNull, bytes.NewReader(content), nil, time.Time{})
		require.NoError(t, err)

		// Ensure the item shows when we list
		listItems, _, err := streamStore.List(ctx, bucketName, "", storj.EncNull, true, 10, meta.None)
		require.NoError(t, err)
		require.True(t, len(listItems) == 1)

		streamID, err := metainfoClient.BeginDeleteObject(ctx, metainfo.BeginDeleteObjectParams{
			Bucket:        []byte(bucketName),
			EncryptedPath: []byte(path),
		})
		require.NoError(t, err)

		segmentItems, _, err := metainfoClient.ListSegments(ctx, metainfo.ListSegmentsParams{
			StreamID: streamID,
			CursorPosition: storj.SegmentPosition{
				Index: 0,
			},
		})
		require.NoError(t, err)

		// We need at least 2 items to do this test
		require.True(t, len(segmentItems) > 1)

		// Delete just the first item
		require.NoError(t, segmentStore.Delete(ctx, streamID, segmentItems[0].Position.Index))

		// It should *still* show when we list, as we've only deleted one
		// segment
		listItems, _, err = streamStore.List(ctx, bucketName, "", storj.EncNull, true, 10, meta.None)
		require.NoError(t, err)
		require.True(t, len(listItems) == 1)

		// Now call the streamStore delete method. This should delete all
		// remaining segments and the file pointer itself without failing
		// because of the missing first segment.
		_ = streamStore.Delete(ctx, fullPath, storj.EncNull)

		// Now it should have 0 list items
		listItems, _, err = streamStore.List(ctx, bucketName, "", storj.EncNull, true, 10, meta.None)
		require.NoError(t, err)
		require.True(t, len(listItems) == 0)
	})
}

func TestStreamStoreList(t *testing.T) {
	runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, streamStore streams.Store) {
		expiration := time.Now().Add(10 * 24 * time.Hour)

		bucketName := "bucket-name"
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		objects := []struct {
			path    string
			content []byte
		}{
			{"aaaa/afile1", []byte("content")},
			{"aaaa/bfile2", []byte("content")},
			{"bbbb/afile1", []byte("content")},
			{"bbbb/bfile2", []byte("content")},
			{"bbbb/bfolder/file1", []byte("content")},
		}
		for _, test := range objects {
			test := test
			data := bytes.NewReader(test.content)
			path := storj.JoinPaths(bucketName, test.path)
			_, err := streamStore.Put(ctx, path, storj.EncNull, data, []byte{}, expiration)
			require.NoError(t, err)
		}

		prefix := bucketName

		// should list all
		items, more, err := streamStore.List(ctx, prefix, "", storj.EncNull, true, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, len(objects), len(items))

		// should list first two and more = true
		items, more, err = streamStore.List(ctx, prefix, "", storj.EncNull, true, 2, meta.None)
		require.NoError(t, err)
		require.True(t, more)
		require.Equal(t, 2, len(items))

		// should list only prefixes
		items, more, err = streamStore.List(ctx, prefix, "", storj.EncNull, false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 2, len(items))

		// should list only BBBB bucket
		prefix = storj.JoinPaths(bucketName, "bbbb")
		items, more, err = streamStore.List(ctx, prefix, "", storj.EncNull, false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 3, len(items))

		// should list only BBBB bucket after afile
		items, more, err = streamStore.List(ctx, prefix, "afile1", storj.EncNull, false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 2, len(items))

		// should list nothing
		prefix = storj.JoinPaths(bucketName, "cccc")
		items, more, err = streamStore.List(ctx, prefix, "", storj.EncNull, true, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 0, len(items))
	})
}

func runTest(t *testing.T, test func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, streamsStore streams.Store)) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		metainfo, _, streamStore := storeTestSetup(t, ctx, planet, 64*memory.MiB.Int64())
		defer ctx.Check(metainfo.Close)
		test(t, ctx, planet, streamStore)
	})
}

func storeTestSetup(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentSize int64) (*metainfo.Client, segments.Store, streams.Store) {
	// TODO move apikey creation to testplanet
	projects, err := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
	require.NoError(t, err)

	apiKey, err := macaroon.NewAPIKey([]byte("testSecret"))
	require.NoError(t, err)

	apiKeyInfo := console.APIKeyInfo{
		ProjectID: projects[0].ID,
		Name:      "testKey",
		Secret:    []byte("testSecret"),
	}

	// add api key to db
	_, err = planet.Satellites[0].DB.Console().APIKeys().Create(context.Background(), apiKey.Head(), apiKeyInfo)
	require.NoError(t, err)

	TestAPIKey := apiKey

	metainfo, err := planet.Uplinks[0].DialMetainfo(context.Background(), planet.Satellites[0], TestAPIKey)
	require.NoError(t, err)

	ec := ecclient.NewClient(planet.Uplinks[0].Log.Named("ecclient"), planet.Uplinks[0].Dialer, 0)

	cfg := planet.Uplinks[0].GetConfig(planet.Satellites[0])
	rs, err := eestream.NewRedundancyStrategyFromStorj(cfg.GetRedundancyScheme())
	require.NoError(t, err)

	segmentStore := segments.NewSegmentStore(metainfo, ec, rs)
	assert.NotNil(t, segmentStore)

	key := new(storj.Key)
	copy(key[:], TestEncKey)

	encStore := encryption.NewStore()
	encStore.SetDefaultKey(key)

	const stripesPerBlock = 2
	blockSize := stripesPerBlock * rs.StripeSize()
	inlineThreshold := 8 * memory.KiB.Int()
	streamStore, err := streams.NewStreamStore(metainfo, segmentStore, segmentSize, encStore, blockSize, storj.EncNull, inlineThreshold, 8*memory.MiB.Int64())
	require.NoError(t, err)

	return metainfo, segmentStore, streamStore
}
