// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	io "io"
	"io/ioutil"
	"testing"
	time "time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/segments"
	storj "storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
	"storj.io/storj/storage"
)

// TODO add more error cases

func TestSegmentStoreMeta(t *testing.T) {
	for _, tt := range []struct {
		pathInput  string
		data       []byte
		metadata   []byte
		expiration time.Time
	}{
		{"l/path/1/2/3", []byte("content"), []byte("metadata"), time.Now().UTC().Add(time.Hour * 12)},
	} {
		runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentStore segments.Store) {
			expectedSize := int64(len(tt.data))
			reader := bytes.NewReader(tt.data)

			beforeModified := time.Now()
			meta, err := segmentStore.Put(ctx, reader, tt.expiration, func() (storj.Path, []byte, error) {
				return tt.pathInput, tt.metadata, nil
			})
			require.NoError(t, err)
			assert.Equal(t, expectedSize, meta.Size)
			assert.Equal(t, tt.metadata, meta.Data)
			assert.Equal(t, tt.expiration, meta.Expiration)
			assert.True(t, meta.Modified.After(beforeModified))

			meta, err = segmentStore.Meta(ctx, tt.pathInput)
			require.NoError(t, err)
			assert.Equal(t, expectedSize, meta.Size)
			assert.Equal(t, tt.metadata, meta.Data)
			assert.Equal(t, tt.expiration, meta.Expiration)
			assert.True(t, meta.Modified.After(beforeModified))
		})
	}
}

func TestSegmentStorePutGetRemote(t *testing.T) {
	for _, tt := range []struct {
		name       string
		path       string
		metadata   []byte
		expiration time.Time
		content    []byte
	}{
		{"test remote put/get", "s0/test_bucket/mypath/1", []byte("metadata"), time.Now().UTC(), createTestData(t, 100*memory.KiB.Int64())},
	} {
		runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentStore segments.Store) {
			_, err := segmentStore.Put(ctx, bytes.NewReader(tt.content), tt.expiration, func() (storj.Path, []byte, error) {
				return tt.path, tt.metadata, nil
			})
			require.NoError(t, err, tt.name)

			_, _, err = segmentStore.Get(ctx, tt.path)
			require.NoError(t, err, tt.name)
		})
	}
}

func TestSegmentStorePutGetInline(t *testing.T) {
	for _, tt := range []struct {
		name       string
		path       string
		metadata   []byte
		expiration time.Time
		content    []byte
	}{
		{"test inline put/get", "l/path/1", []byte("metadata"), time.Now().UTC(), createTestData(t, 2*memory.KiB.Int64())},
	} {
		runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentStore segments.Store) {
			_, err := segmentStore.Put(ctx, bytes.NewReader(tt.content), tt.expiration, func() (storj.Path, []byte, error) {
				return tt.path, tt.metadata, nil
			})
			require.NoError(t, err, tt.name)

			_, _, err = segmentStore.Get(ctx, tt.path)
			require.NoError(t, err, tt.name)
		})
	}
}

func TestSegmentStoreDeleteInline(t *testing.T) {
	for _, tt := range []struct {
		name       string
		path       string
		metadata   []byte
		expiration time.Time
		content    []byte
	}{
		{"test inline delete", "l/path/1", []byte("metadata"), time.Now().UTC(), createTestData(t, 2*memory.KiB.Int64())},
	} {
		runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentStore segments.Store) {
			_, err := segmentStore.Put(ctx, bytes.NewReader(tt.content), tt.expiration, func() (storj.Path, []byte, error) {
				return tt.path, tt.metadata, nil
			})
			require.NoError(t, err, tt.name)

			_, _, err = segmentStore.Get(ctx, tt.path)
			require.NoError(t, err, tt.name)

			err = segmentStore.Delete(ctx, tt.path)
			require.NoError(t, err, tt.name)

			_, _, err = segmentStore.Get(ctx, tt.path)
			require.Error(t, err, tt.name)
			require.True(t, storage.ErrKeyNotFound.Has(err))
		})
	}
}

func TestSegmentStoreDeleteRemote(t *testing.T) {
	for _, tt := range []struct {
		name       string
		path       string
		metadata   []byte
		expiration time.Time
		content    []byte
	}{
		{"test remote delete", "s0/test_bucket/mypath/1", []byte("metadata"), time.Now().UTC(), createTestData(t, 100*memory.KiB.Int64())},
	} {
		runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentStore segments.Store) {

			_, err := segmentStore.Put(ctx, bytes.NewReader(tt.content), tt.expiration, func() (storj.Path, []byte, error) {
				return tt.path, tt.metadata, nil
			})
			require.NoError(t, err, tt.name)

			_, _, err = segmentStore.Get(ctx, tt.path)
			require.NoError(t, err, tt.name)

			err = segmentStore.Delete(ctx, tt.path)
			require.NoError(t, err, tt.name)

			_, _, err = segmentStore.Get(ctx, tt.path)
			require.Error(t, err, tt.name)
			require.True(t, storage.ErrKeyNotFound.Has(err))
		})
	}
}

func TestSegmentStoreList(t *testing.T) {
	runTest(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentStore segments.Store) {

		expiration := time.Now().Add(24 * time.Hour * 10)

		segments := []struct {
			path    string
			content []byte
		}{
			{"l/AAAA/afile1", []byte("content")},
			{"l/AAAA/bfile2", []byte("content")},
			{"l/BBBB/afile1", []byte("content")},
			{"l/BBBB/bfile2", []byte("content")},
			{"l/BBBB/bfolder/file1", []byte("content")},
		}
		for _, segment := range segments {
			_, err := segmentStore.Put(ctx, bytes.NewReader(segment.content), expiration, func() (storj.Path, []byte, error) {
				return segment.path, []byte{}, nil
			})
			require.NoError(t, err)
		}

		// should list all
		items, more, err := segmentStore.List(ctx, "l", "", "", true, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, len(segments), len(items))

		// should list first two and more = true
		items, more, err = segmentStore.List(ctx, "l", "", "", true, 2, meta.None)
		require.NoError(t, err)
		require.True(t, more)
		require.Equal(t, 2, len(items))

		// should list only prefixes
		items, more, err = segmentStore.List(ctx, "l", "", "", false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 2, len(items))

		// should list only BBBB bucket
		items, more, err = segmentStore.List(ctx, "l/BBBB", "", "", false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 3, len(items))

		// should list only BBBB bucket after afile1
		items, more, err = segmentStore.List(ctx, "l/BBBB", "afile1", "", false, 10, meta.None)
		require.NoError(t, err)
		require.False(t, more)
		require.Equal(t, 2, len(items))
	})
}

func TestCalcNeededNodes(t *testing.T) {
	for i, tt := range []struct {
		k, m, o, n int32
		needed     int32
	}{
		{k: 0, m: 0, o: 0, n: 0, needed: 0},
		{k: 1, m: 1, o: 1, n: 1, needed: 1},
		{k: 1, m: 1, o: 2, n: 2, needed: 2},
		{k: 1, m: 2, o: 2, n: 2, needed: 2},
		{k: 2, m: 3, o: 4, n: 4, needed: 3},
		{k: 2, m: 4, o: 6, n: 8, needed: 3},
		{k: 20, m: 30, o: 40, n: 50, needed: 25},
		{k: 29, m: 35, o: 80, n: 95, needed: 34},
	} {
		tag := fmt.Sprintf("#%d. %+v", i, tt)

		rs := pb.RedundancyScheme{
			MinReq:           tt.k,
			RepairThreshold:  tt.m,
			SuccessThreshold: tt.o,
			Total:            tt.n,
		}

		assert.Equal(t, tt.needed, segments.CalcNeededNodes(&rs), tag)
	}
}

func createTestData(t *testing.T, size int64) []byte {
	data, err := ioutil.ReadAll(io.LimitReader(rand.Reader, size))
	require.NoError(t, err)
	return data
}

func runTest(t *testing.T, test func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, segmentStore segments.Store)) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// TODO move apikey creation to testplanet
		project, err := planet.Satellites[0].DB.Console().Projects().Insert(context.Background(), &console.Project{
			Name: "testProject",
		})
		require.NoError(t, err)

		apiKey := console.APIKey{}
		apiKeyInfo := console.APIKeyInfo{
			ProjectID: project.ID,
			Name:      "testKey",
		}

		// add api key to db
		_, err = planet.Satellites[0].DB.Console().APIKeys().Create(context.Background(), apiKey, apiKeyInfo)
		require.NoError(t, err)

		TestAPIKey := apiKey.String()

		oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
		require.NoError(t, err)

		metainfo, err := planet.Uplinks[0].DialMetainfo(context.Background(), planet.Satellites[0], TestAPIKey)
		require.NoError(t, err)

		ec := ecclient.NewClient(planet.Uplinks[0].Transport, 0)
		fc, err := infectious.NewFEC(2, 4)
		require.NoError(t, err)

		rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, 1*memory.KiB.Int()), 0, 0)
		require.NoError(t, err)

		segmentStore := segments.NewSegmentStore(metainfo, oc, ec, rs, 4*memory.KiB.Int(), 8*memory.MiB.Int64())
		assert.NotNil(t, segmentStore)

		test(t, ctx, planet, segmentStore)
	})
}
