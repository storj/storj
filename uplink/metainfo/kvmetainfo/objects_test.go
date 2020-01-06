// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/encryption"
	"storj.io/common/memory"
	"storj.io/common/paths"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/uplink/metainfo/kvmetainfo"
	"storj.io/storj/uplink/storage/streams"
	"storj.io/storj/uplink/stream"
)

const TestFile = "test-file"

func TestCreateObject(t *testing.T) {
	customRS := storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		RequiredShares: 29,
		RepairShares:   35,
		OptimalShares:  80,
		TotalShares:    95,
		ShareSize:      2 * memory.KiB.Int32(),
	}

	const stripesPerBlock = 2
	customEP := storj.EncryptionParameters{
		CipherSuite: storj.EncNull,
		BlockSize:   stripesPerBlock * customRS.StripeSize(),
	}

	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		require.NoError(t, err)

		for i, tt := range []struct {
			create     *kvmetainfo.CreateObject
			expectedRS storj.RedundancyScheme
			expectedEP storj.EncryptionParameters
		}{
			{
				create:     nil,
				expectedRS: kvmetainfo.DefaultRS,
				expectedEP: kvmetainfo.DefaultES,
			},
			{
				create:     &kvmetainfo.CreateObject{RedundancyScheme: customRS, EncryptionParameters: customEP},
				expectedRS: customRS,
				expectedEP: customEP,
			},
			{
				create:     &kvmetainfo.CreateObject{RedundancyScheme: customRS},
				expectedRS: customRS,
				expectedEP: storj.EncryptionParameters{CipherSuite: kvmetainfo.DefaultES.CipherSuite, BlockSize: kvmetainfo.DefaultES.BlockSize},
			},
			{
				create:     &kvmetainfo.CreateObject{EncryptionParameters: customEP},
				expectedRS: kvmetainfo.DefaultRS,
				expectedEP: storj.EncryptionParameters{CipherSuite: customEP.CipherSuite, BlockSize: kvmetainfo.DefaultES.BlockSize},
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			obj, err := db.CreateObject(ctx, bucket, TestFile, tt.create)
			require.NoError(t, err)

			info := obj.Info()

			assert.Equal(t, TestBucket, info.Bucket.Name, errTag)
			assert.Equal(t, storj.EncAESGCM, info.Bucket.PathCipher, errTag)
			assert.Equal(t, TestFile, info.Path, errTag)
			assert.EqualValues(t, 0, info.Size, errTag)
			assert.Equal(t, tt.expectedRS, info.RedundancyScheme, errTag)
			assert.Equal(t, tt.expectedEP, info.EncryptionParameters, errTag)
		}
	})
}

func TestGetObject(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		require.NoError(t, err)
		upload(ctx, t, db, streams, bucket, TestFile, nil)

		_, err = db.GetObject(ctx, storj.Bucket{}, "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.GetObject(ctx, bucket, "")
		assert.True(t, storj.ErrNoPath.Has(err))

		nonExistingBucket := storj.Bucket{
			Name:       "non-existing-bucket",
			PathCipher: storj.EncNull,
		}
		_, err = db.GetObject(ctx, nonExistingBucket, TestFile)
		assert.True(t, storj.ErrObjectNotFound.Has(err))

		_, err = db.GetObject(ctx, bucket, "non-existing-file")
		assert.True(t, storj.ErrObjectNotFound.Has(err))

		object, err := db.GetObject(ctx, bucket, TestFile)
		if assert.NoError(t, err) {
			assert.Equal(t, TestFile, object.Path)
			assert.Equal(t, TestBucket, object.Bucket.Name)
			assert.Equal(t, storj.EncAESGCM, object.Bucket.PathCipher)
		}
	})
}

func TestGetObjectStream(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
		data := testrand.Bytes(32 * memory.KiB)

		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		require.NoError(t, err)

		emptyFile := upload(ctx, t, db, streams, bucket, "empty-file", nil)
		smallFile := upload(ctx, t, db, streams, bucket, "small-file", []byte("test"))
		largeFile := upload(ctx, t, db, streams, bucket, "large-file", data)

		emptyBucket := storj.Bucket{
			PathCipher: storj.EncNull,
		}
		_, err = db.GetObjectStream(ctx, emptyBucket, storj.Object{})
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.GetObjectStream(ctx, bucket, storj.Object{})
		assert.True(t, storj.ErrNoPath.Has(err))

		nonExistingBucket := storj.Bucket{
			Name:       "non-existing-bucket",
			PathCipher: storj.EncNull,
		}

		// no error because we are not doing satellite connection with this method
		_, err = db.GetObjectStream(ctx, nonExistingBucket, smallFile)
		assert.NoError(t, err)

		// no error because we are not doing satellite connection with this method
		_, err = db.GetObjectStream(ctx, bucket, storj.Object{
			Path: "non-existing-file",
		})
		assert.NoError(t, err)

		assertStream(ctx, t, db, streams, bucket, emptyFile, []byte{})
		assertStream(ctx, t, db, streams, bucket, smallFile, []byte("test"))
		assertStream(ctx, t, db, streams, bucket, largeFile, data)

		/* TODO: Disable stopping due to flakiness.
		// Stop randomly half of the storage nodes and remove them from satellite's overlay
		perm := mathrand.Perm(len(planet.StorageNodes))
		for _, i := range perm[:(len(perm) / 2)] {
			assert.NoError(t, planet.StopPeer(planet.StorageNodes[i]))
			_, err := planet.Satellites[0].Overlay.Service.UpdateUptime(ctx, planet.StorageNodes[i].ID(), false)
			assert.NoError(t, err)
		}

		// try downloading the large file again
		assertStream(ctx, t, db, streams, bucket, "large-file", 32*memory.KiB.Int64(), data)
		*/
	})
}

func upload(ctx context.Context, t *testing.T, db *kvmetainfo.DB, streams streams.Store, bucket storj.Bucket, path storj.Path, data []byte) storj.Object {
	obj, err := db.CreateObject(ctx, bucket, path, nil)
	require.NoError(t, err)

	str, err := obj.CreateStream(ctx)
	require.NoError(t, err)

	upload := stream.NewUpload(ctx, str, streams)

	_, err = upload.Write(data)
	require.NoError(t, err)

	err = upload.Close()
	require.NoError(t, err)

	err = obj.Commit(ctx)
	require.NoError(t, err)

	return obj.Info()
}

func assertStream(ctx context.Context, t *testing.T, db *kvmetainfo.DB, streams streams.Store, bucket storj.Bucket, object storj.Object, content []byte) {
	readOnly, err := db.GetObjectStream(ctx, bucket, object)
	require.NoError(t, err)

	assert.Equal(t, object.Path, readOnly.Info().Path)
	assert.Equal(t, TestBucket, readOnly.Info().Bucket.Name)
	assert.Equal(t, storj.EncAESGCM, readOnly.Info().Bucket.PathCipher)

	segments, more, err := readOnly.Segments(ctx, 0, 0)
	require.NoError(t, err)

	assert.False(t, more)
	if !assert.Equal(t, 1, len(segments)) {
		return
	}

	assert.EqualValues(t, 0, segments[0].Index)
	assert.EqualValues(t, len(content), segments[0].Size)
	if segments[0].Size > 4*memory.KiB.Int64() {
		assertRemoteSegment(t, segments[0])
	} else {
		assertInlineSegment(t, segments[0], content)
	}

	download := stream.NewDownload(ctx, readOnly, streams)
	defer func() {
		err = download.Close()
		assert.NoError(t, err)
	}()

	data := make([]byte, len(content))
	n, err := io.ReadFull(download, data)
	require.NoError(t, err)

	assert.Equal(t, len(content), n)
	assert.Equal(t, content, data)
}

func assertInlineSegment(t *testing.T, segment storj.Segment, content []byte) {
	assert.Equal(t, content, segment.Inline)
	assert.True(t, segment.PieceID.IsZero())
	assert.Equal(t, 0, len(segment.Pieces))
}

func assertRemoteSegment(t *testing.T, segment storj.Segment) {
	assert.Nil(t, segment.Inline)
	assert.NotNil(t, segment.PieceID)

	// check that piece numbers and nodes are unique
	nums := make(map[byte]struct{})
	nodes := make(map[string]struct{})
	for _, piece := range segment.Pieces {
		if _, ok := nums[piece.Number]; ok {
			t.Fatalf("piece number %d is not unique", piece.Number)
		}
		nums[piece.Number] = struct{}{}

		id := piece.Location.String()
		if _, ok := nodes[id]; ok {
			t.Fatalf("node id %s is not unique", id)
		}
		nodes[id] = struct{}{}
	}
}

func TestDeleteObject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		encStore := newTestEncStore(TestEncKey)
		db, streams, err := newMetainfoParts(planet, encStore)
		require.NoError(t, err)

		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		unencryptedPath := paths.NewUnencrypted(TestFile)
		encryptedPath, err := encryption.EncryptPath(bucket.Name, unencryptedPath, storj.EncAESGCM, encStore)
		require.NoError(t, err)

		for i, path := range []string{unencryptedPath.String(), encryptedPath.String()} {
			upload(ctx, t, db, streams, bucket, path, nil)

			if i < 0 {
				// Enable encryption bypass
				encStore.EncryptionBypass = true
			}

			err = db.DeleteObject(ctx, storj.Bucket{}, "")
			assert.True(t, storj.ErrNoBucket.Has(err))

			err = db.DeleteObject(ctx, bucket, "")
			assert.True(t, storj.ErrNoPath.Has(err))

			{
				unexistingBucket := storj.Bucket{
					Name:       bucket.Name + "-not-exist",
					PathCipher: bucket.PathCipher,
				}
				err = db.DeleteObject(ctx, unexistingBucket, TestFile)
				assert.True(t, storj.ErrObjectNotFound.Has(err))
			}

			err = db.DeleteObject(ctx, bucket, "non-existing-file")
			assert.True(t, storj.ErrObjectNotFound.Has(err))

			{
				invalidPathCipherBucket := storj.Bucket{
					Name:       bucket.Name,
					PathCipher: bucket.PathCipher + 1,
				}
				err = db.DeleteObject(ctx, invalidPathCipherBucket, TestFile)
				assert.True(t, storj.ErrObjectNotFound.Has(err))
			}

			err = db.DeleteObject(ctx, bucket, path)
			assert.NoError(t, err)
		}
	})
}

func TestListObjectsEmpty(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
		testBucketInfo, err := db.CreateBucket(ctx, TestBucket, nil)
		require.NoError(t, err)

		_, err = db.ListObjects(ctx, storj.Bucket{}, storj.ListOptions{})
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.ListObjects(ctx, testBucketInfo, storj.ListOptions{})
		assert.EqualError(t, err, "kvmetainfo: invalid direction 0")

		// TODO for now we are supporting only storj.After
		for _, direction := range []storj.ListDirection{
			// storj.Forward,
			storj.After,
		} {
			list, err := db.ListObjects(ctx, testBucketInfo, storj.ListOptions{Direction: direction})
			if assert.NoError(t, err) {
				assert.False(t, list.More)
				assert.Equal(t, 0, len(list.Items))
			}
		}
	})
}

func TestListObjects_EncryptionBypass(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		encStore := newTestEncStore(TestEncKey)
		db, streams, err := newMetainfoParts(planet, encStore)
		require.NoError(t, err)

		bucket, err := db.CreateBucket(ctx, TestBucket, &storj.Bucket{PathCipher: storj.EncAESGCM})
		require.NoError(t, err)

		filePaths := []string{
			"a", "aa", "b", "bb", "c",
			"a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc",
			"b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc",
		}

		for _, path := range filePaths {
			upload(ctx, t, db, streams, bucket, path, nil)
		}
		sort.Strings(filePaths)

		// Enable encryption bypass
		encStore.EncryptionBypass = true

		opts := options("", "", 0)
		opts.Recursive = true
		encodedList, err := db.ListObjects(ctx, bucket, opts)
		require.NoError(t, err)
		require.Equal(t, len(filePaths), len(encodedList.Items))

		seenPaths := make(map[string]struct{})
		for _, item := range encodedList.Items {
			iter := paths.NewUnencrypted(item.Path).Iterator()
			var decoded, next string
			for !iter.Done() {
				next = iter.Next()

				decodedNextBytes, err := base64.URLEncoding.DecodeString(next)
				require.NoError(t, err)

				decoded += string(decodedNextBytes) + "/"
			}
			decoded = strings.TrimRight(decoded, "/")
			encryptedPath := paths.NewEncrypted(decoded)

			decryptedPath, err := encryption.DecryptPath(bucket.Name, encryptedPath, storj.EncAESGCM, encStore)
			require.NoError(t, err)

			// NB: require decrypted path is a member of `filePaths`.
			result := sort.Search(len(filePaths), func(i int) bool {
				return !paths.NewUnencrypted(filePaths[i]).Less(decryptedPath)
			})
			require.NotEqual(t, len(filePaths), result)

			// NB: ensure each path is only seen once.
			_, ok := seenPaths[decryptedPath.String()]
			require.False(t, ok)

			seenPaths[decryptedPath.String()] = struct{}{}
		}
	})
}

func TestListObjects(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, &storj.Bucket{PathCipher: storj.EncNull})
		require.NoError(t, err)

		filePaths := []string{
			"a", "aa", "b", "bb", "c",
			"a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc",
			"b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc",
		}

		for _, path := range filePaths {
			upload(ctx, t, db, streams, bucket, path, nil)
		}

		otherBucket, err := db.CreateBucket(ctx, "otherbucket", nil)
		require.NoError(t, err)

		upload(ctx, t, db, streams, otherBucket, "file-in-other-bucket", nil)

		for i, tt := range []struct {
			options storj.ListOptions
			more    bool
			result  []string
		}{
			{
				options: options("", "", 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "`", 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "b", 0),
				result:  []string{"b/", "bb", "c"},
			}, {
				options: options("", "c", 0),
				result:  []string{},
			}, {
				options: options("", "ca", 0),
				result:  []string{},
			}, {
				options: options("", "", 1),
				more:    true,
				result:  []string{"a"},
			}, {
				options: options("", "`", 1),
				more:    true,
				result:  []string{"a"},
			}, {
				options: options("", "aa", 1),
				more:    true,
				result:  []string{"b"},
			}, {
				options: options("", "c", 1),
				result:  []string{},
			}, {
				options: options("", "ca", 1),
				result:  []string{},
			}, {
				options: options("", "", 2),
				more:    true,
				result:  []string{"a", "a/"},
			}, {
				options: options("", "`", 2),
				more:    true,
				result:  []string{"a", "a/"},
			}, {
				options: options("", "aa", 2),
				more:    true,
				result:  []string{"b", "b/"},
			}, {
				options: options("", "bb", 2),
				result:  []string{"c"},
			}, {
				options: options("", "c", 2),
				result:  []string{},
			}, {
				options: options("", "ca", 2),
				result:  []string{},
			}, {
				options: optionsRecursive("", "", 0),
				result:  []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			}, {
				options: options("a", "", 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "", 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "xb", 0),
				result:  []string{"xbb", "xc"},
			}, {
				options: optionsRecursive("", "a/xbb", 5),
				more:    true,
				result:  []string{"a/xc", "aa", "b", "b/ya", "b/yaa"},
			}, {
				options: options("a/", "xaa", 2),
				more:    true,
				result:  []string{"xb", "xbb"},
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			list, err := db.ListObjects(ctx, bucket, tt.options)

			if assert.NoError(t, err, errTag) {
				assert.Equal(t, tt.more, list.More, errTag)
				for i, item := range list.Items {
					assert.Equal(t, tt.result[i], item.Path, errTag)
					assert.Equal(t, TestBucket, item.Bucket.Name, errTag)
					assert.Equal(t, storj.EncNull, item.Bucket.PathCipher, errTag)
				}
			}
		}
	})
}

func options(prefix, cursor string, limit int) storj.ListOptions {
	return storj.ListOptions{
		Prefix:    prefix,
		Cursor:    cursor,
		Direction: storj.After,
		Limit:     limit,
	}
}

func optionsRecursive(prefix, cursor string, limit int) storj.ListOptions {
	return storj.ListOptions{
		Prefix:    prefix,
		Cursor:    cursor,
		Direction: storj.After,
		Limit:     limit,
		Recursive: true,
	}
}
