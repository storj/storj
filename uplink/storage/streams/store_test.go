// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
	"storj.io/storj/uplink/ecclient"
	"storj.io/storj/uplink/eestream"
	"storj.io/storj/uplink/storage/meta"
	"storj.io/storj/uplink/storage/segments"
	"storj.io/storj/uplink/storage/streams"
)

const (
	TestEncKey = "test-encryption-key"
)

func TestStreamsStorePutGet(t *testing.T) {
	runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, streamStore streams.Store) {
		bucketName := "bucket-name"
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		for _, tt := range []struct {
			name       string
			path       string
			metadata   []byte
			expiration time.Time
			content    []byte
		}{
			{"test inline put/get", "path/1", []byte("inline-metadata"), time.Time{}, testrand.Bytes(2 * memory.KiB)},
			{"test remote put/get", "mypath/1", []byte("remote-metadata"), time.Time{}, testrand.Bytes(100 * memory.KiB)},
		} {
			test := tt

			path := storj.JoinPaths(bucketName, test.path)
			_, err = streamStore.Put(ctx, path, storj.EncNull, bytes.NewReader(test.content), test.metadata, test.expiration)
			require.NoError(t, err, test.name)

			rr, metadata, err := streamStore.Get(ctx, path, storj.EncNull)
			require.NoError(t, err, test.name)
			require.Equal(t, test.metadata, metadata.Data)

			reader, err := rr.Range(ctx, 0, rr.Size())
			require.NoError(t, err, test.name)
			content, err := ioutil.ReadAll(reader)
			require.NoError(t, err, test.name)
			require.Equal(t, test.content, content)

			require.NoError(t, reader.Close(), test.name)
		}
	})
}

func TestStreamsStoreDelete(t *testing.T) {
	runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, streamStore streams.Store) {
		bucketName := "bucket-name"
		err := planet.Uplinks[0].CreateBucket(ctx, planet.Satellites[0], bucketName)
		require.NoError(t, err)

		for _, tt := range []struct {
			name       string
			path       string
			metadata   []byte
			expiration time.Time
			content    []byte
		}{
			{"test inline delete", "path/1", []byte("inline-metadata"), time.Time{}, testrand.Bytes(2 * memory.KiB)},
			{"test remote delete", "mypath/1", []byte("remote-metadata"), time.Time{}, testrand.Bytes(100 * memory.KiB)},
		} {
			test := tt

			path := storj.JoinPaths(bucketName, test.path)
			_, err = streamStore.Put(ctx, path, storj.EncNull, bytes.NewReader(test.content), test.metadata, test.expiration)
			require.NoError(t, err, test.name)

			// delete existing
			err = streamStore.Delete(ctx, path, storj.EncNull)
			require.NoError(t, err, test.name)

			_, _, err = streamStore.Get(ctx, path, storj.EncNull)
			require.Error(t, err, test.name)
			require.True(t, storj.ErrObjectNotFound.Has(err))

			// delete non existing
			err = streamStore.Delete(ctx, path, storj.EncNull)
			require.Error(t, err, test.name)
			require.True(t, storj.ErrObjectNotFound.Has(err))
		}
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
		items, more, err := streamStore.List(ctx, prefix, "", "", storj.EncNull, true, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, len(objects), len(items))

		// should list first two and more = true
		items, more, err = streamStore.List(ctx, prefix, "", "", storj.EncNull, true, 2, meta.None)
		require.NoError(t, err)
		require.True(t, more)
		require.Equal(t, 2, len(items))

		// should list only prefixes
		items, more, err = streamStore.List(ctx, prefix, "", "", storj.EncNull, false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 2, len(items))

		// should list only BBBB bucket
		prefix = storj.JoinPaths(bucketName, "bbbb")
		items, more, err = streamStore.List(ctx, prefix, "", "", storj.EncNull, false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 3, len(items))

		// should list only BBBB bucket after afile
		items, more, err = streamStore.List(ctx, prefix, "afile1", "", storj.EncNull, false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 2, len(items))

		// should list nothing
		prefix = storj.JoinPaths(bucketName, "cccc")
		items, more, err = streamStore.List(ctx, prefix, "", "", storj.EncNull, true, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 0, len(items))
	})
}

func runTest(t *testing.T, test func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, streamsStore streams.Store)) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
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

		TestAPIKey := apiKey.Serialize()

		metainfo, err := planet.Uplinks[0].DialMetainfo(context.Background(), planet.Satellites[0], TestAPIKey)
		require.NoError(t, err)
		defer ctx.Check(metainfo.Close)

		ec := ecclient.NewClient(planet.Uplinks[0].Log.Named("ecclient"), planet.Uplinks[0].Transport, 0)

		cfg := planet.Uplinks[0].GetConfig(planet.Satellites[0])
		rs, err := eestream.NewRedundancyStrategyFromStorj(cfg.GetRedundancyScheme())
		require.NoError(t, err)

		segmentStore := segments.NewSegmentStore(metainfo, ec, rs, 4*memory.KiB.Int(), 8*memory.MiB.Int64())
		assert.NotNil(t, segmentStore)

		key := new(storj.Key)
		copy(key[:], TestEncKey)

		encStore := encryption.NewStore()
		encStore.SetDefaultKey(key)

		const stripesPerBlock = 2
		blockSize := stripesPerBlock * rs.StripeSize()
		inlineThreshold := 8 * memory.KiB.Int()
		streamStore, err := streams.NewStreamStore(metainfo, segmentStore, 64*memory.MiB.Int64(), encStore, blockSize, storj.EncNull, inlineThreshold)
		require.NoError(t, err)

		test(t, ctx, planet, streamStore)
	})
}
