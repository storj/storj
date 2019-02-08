// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
)

const TestFile = "test-file"

func TestCreateObject(t *testing.T) {
	customRS := storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		RequiredShares: 29,
		RepairShares:   35,
		OptimalShares:  80,
		TotalShares:    95,
		ShareSize:      2 * memory.KB.Int32(),
	}

	customES := storj.EncryptionScheme{
		Cipher:    storj.Unencrypted,
		BlockSize: 1 * memory.KB.Int32(),
	}

	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		for i, tt := range []struct {
			create     *storj.CreateObject
			expectedRS storj.RedundancyScheme
			expectedES storj.EncryptionScheme
		}{
			{
				create:     nil,
				expectedRS: kvmetainfo.DefaultRS,
				expectedES: kvmetainfo.DefaultES,
			}, {
				create:     &storj.CreateObject{RedundancyScheme: customRS, EncryptionScheme: customES},
				expectedRS: customRS,
				expectedES: customES,
			}, {
				create:     &storj.CreateObject{RedundancyScheme: customRS},
				expectedRS: customRS,
				expectedES: storj.EncryptionScheme{Cipher: kvmetainfo.DefaultES.Cipher, BlockSize: customRS.ShareSize},
			}, {
				create:     &storj.CreateObject{EncryptionScheme: customES},
				expectedRS: kvmetainfo.DefaultRS,
				expectedES: customES,
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			obj, err := db.CreateObject(ctx, bucket.Name, TestFile, tt.create)
			if !assert.NoError(t, err) {
				return
			}

			info := obj.Info()

			assert.Equal(t, TestBucket, info.Bucket.Name, errTag)
			assert.Equal(t, storj.AESGCM, info.Bucket.PathCipher, errTag)
			assert.Equal(t, TestFile, info.Path, errTag)
			assert.EqualValues(t, 0, info.Size, errTag)
			assert.Equal(t, tt.expectedRS, info.RedundancyScheme, errTag)
			assert.Equal(t, tt.expectedES, info.EncryptionScheme, errTag)
		}
	})
}

func TestGetObject(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		upload(ctx, t, db, streams, bucket, TestFile, nil)

		_, err = db.GetObject(ctx, "", "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.GetObject(ctx, bucket.Name, "")
		assert.True(t, storj.ErrNoPath.Has(err))

		_, err = db.GetObject(ctx, "non-existing-bucket", TestFile)
		assert.True(t, storj.ErrBucketNotFound.Has(err))

		_, err = db.GetObject(ctx, bucket.Name, "non-existing-file")
		assert.True(t, storj.ErrObjectNotFound.Has(err))

		object, err := db.GetObject(ctx, bucket.Name, TestFile)
		if assert.NoError(t, err) {
			assert.Equal(t, TestFile, object.Path)
			assert.Equal(t, TestBucket, object.Bucket.Name)
			assert.Equal(t, storj.AESGCM, object.Bucket.PathCipher)
		}
	})
}

func TestGetObjectStream(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		data := make([]byte, 32*memory.KB)
		_, err := rand.Read(data)
		if !assert.NoError(t, err) {
			return
		}

		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		upload(ctx, t, db, streams, bucket, "empty-file", nil)
		upload(ctx, t, db, streams, bucket, "small-file", []byte("test"))
		upload(ctx, t, db, streams, bucket, "large-file", data)

		_, err = db.GetObjectStream(ctx, "", "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.GetObjectStream(ctx, bucket.Name, "")
		assert.True(t, storj.ErrNoPath.Has(err))

		_, err = db.GetObjectStream(ctx, "non-existing-bucket", "small-file")
		assert.True(t, storj.ErrBucketNotFound.Has(err))

		_, err = db.GetObjectStream(ctx, bucket.Name, "non-existing-file")
		assert.True(t, storj.ErrObjectNotFound.Has(err))

		assertStream(ctx, t, db, streams, bucket, "empty-file", 0, []byte{})
		assertStream(ctx, t, db, streams, bucket, "small-file", 4, []byte("test"))
		assertStream(ctx, t, db, streams, bucket, "large-file", 32*memory.KB.Int64(), data)

		/* TODO: Disable stopping due to flakiness.
		// Stop randomly half of the storage nodes and remove them from satellite's overlay cache
		perm := mathrand.Perm(len(planet.StorageNodes))
		for _, i := range perm[:(len(perm) / 2)] {
			assert.NoError(t, planet.StopPeer(planet.StorageNodes[i]))
			assert.NoError(t, planet.Satellites[0].Overlay.Service.Delete(ctx, planet.StorageNodes[i].ID()))
		}

		// try downloading the large file again
		assertStream(ctx, t, db, streams, bucket, "large-file", 32*memory.KB.Int64(), data)
		*/
	})
}

func upload(ctx context.Context, t *testing.T, db *kvmetainfo.DB, streams streams.Store, bucket storj.Bucket, path storj.Path, data []byte) {
	obj, err := db.CreateObject(ctx, bucket.Name, path, nil)
	if !assert.NoError(t, err) {
		return
	}

	str, err := obj.CreateStream(ctx)
	if !assert.NoError(t, err) {
		return
	}

	upload := stream.NewUpload(ctx, str, streams)

	_, err = upload.Write(data)
	if !assert.NoError(t, err) {
		return
	}

	err = upload.Close()
	if !assert.NoError(t, err) {
		return
	}

	err = obj.Commit(ctx)
	if !assert.NoError(t, err) {
		return
	}
}

func assertStream(ctx context.Context, t *testing.T, db *kvmetainfo.DB, streams streams.Store, bucket storj.Bucket, path storj.Path, size int64, content []byte) {
	readOnly, err := db.GetObjectStream(ctx, bucket.Name, path)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, path, readOnly.Info().Path)
	assert.Equal(t, TestBucket, readOnly.Info().Bucket.Name)
	assert.Equal(t, storj.AESGCM, readOnly.Info().Bucket.PathCipher)

	segments, more, err := readOnly.Segments(ctx, 0, 0)
	if !assert.NoError(t, err) {
		return
	}

	assert.False(t, more)
	if !assert.Equal(t, 1, len(segments)) {
		return
	}

	assert.EqualValues(t, 0, segments[0].Index)
	assert.EqualValues(t, len(content), segments[0].Size)
	if segments[0].Size > 4*memory.KB.Int64() {
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
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, len(content), n)
	assert.Equal(t, content, data)
}

func assertInlineSegment(t *testing.T, segment storj.Segment, content []byte) {
	assert.Equal(t, content, segment.Inline)
	assert.Nil(t, segment.PieceID)
	assert.Equal(t, 0, len(segment.Pieces))
}

func assertRemoteSegment(t *testing.T, segment storj.Segment) {
	assert.Nil(t, segment.Inline)
	assert.NotNil(t, segment.PieceID)
	assert.NotEqual(t, 0, len(segment.Pieces))

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
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		upload(ctx, t, db, streams, bucket, TestFile, nil)

		err = db.DeleteObject(ctx, "", "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		err = db.DeleteObject(ctx, bucket.Name, "")
		assert.True(t, storj.ErrNoPath.Has(err))

		err = db.DeleteObject(ctx, "non-existing-bucket", TestFile)
		assert.True(t, storj.ErrBucketNotFound.Has(err))

		err = db.DeleteObject(ctx, bucket.Name, "non-existing-file")
		assert.True(t, storj.ErrObjectNotFound.Has(err))

		err = db.DeleteObject(ctx, bucket.Name, TestFile)
		assert.NoError(t, err)
	})
}

func TestListObjectsEmpty(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		_, err = db.ListObjects(ctx, "", storj.ListOptions{})
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.ListObjects(ctx, bucket.Name, storj.ListOptions{})
		assert.EqualError(t, err, "kvmetainfo: invalid direction 0")

		for _, direction := range []storj.ListDirection{
			storj.Before,
			storj.Backward,
			storj.Forward,
			storj.After,
		} {
			list, err := db.ListObjects(ctx, bucket.Name, storj.ListOptions{Direction: direction})
			if assert.NoError(t, err) {
				assert.False(t, list.More)
				assert.Equal(t, 0, len(list.Items))
			}
		}
	})
}

func TestListObjects(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		bucket, err := db.CreateBucket(ctx, TestBucket, &storj.Bucket{PathCipher: storj.Unencrypted})
		if !assert.NoError(t, err) {
			return
		}

		filePaths := []string{
			"a", "aa", "b", "bb", "c",
			"a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc",
			"b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc",
		}

		for _, path := range filePaths {
			upload(ctx, t, db, streams, bucket, path, nil)
		}

		otherBucket, err := db.CreateBucket(ctx, "otherbucket", nil)
		if !assert.NoError(t, err) {
			return
		}

		upload(ctx, t, db, streams, otherBucket, "file-in-other-bucket", nil)

		for i, tt := range []struct {
			options storj.ListOptions
			more    bool
			result  []string
		}{
			{
				options: options("", "", storj.After, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "`", storj.After, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "b", storj.After, 0),
				result:  []string{"b/", "bb", "c"},
			}, {
				options: options("", "c", storj.After, 0),
				result:  []string{},
			}, {
				options: options("", "ca", storj.After, 0),
				result:  []string{},
			}, {
				options: options("", "", storj.After, 1),
				more:    true,
				result:  []string{"a"},
			}, {
				options: options("", "`", storj.After, 1),
				more:    true,
				result:  []string{"a"},
			}, {
				options: options("", "aa", storj.After, 1),
				more:    true,
				result:  []string{"b"},
			}, {
				options: options("", "c", storj.After, 1),
				result:  []string{},
			}, {
				options: options("", "ca", storj.After, 1),
				result:  []string{},
			}, {
				options: options("", "", storj.After, 2),
				more:    true,
				result:  []string{"a", "a/"},
			}, {
				options: options("", "`", storj.After, 2),
				more:    true,
				result:  []string{"a", "a/"},
			}, {
				options: options("", "aa", storj.After, 2),
				more:    true,
				result:  []string{"b", "b/"},
			}, {
				options: options("", "bb", storj.After, 2),
				result:  []string{"c"},
			}, {
				options: options("", "c", storj.After, 2),
				result:  []string{},
			}, {
				options: options("", "ca", storj.After, 2),
				result:  []string{},
			}, {
				options: optionsRecursive("", "", storj.After, 0),
				result:  []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			}, {
				options: options("a", "", storj.After, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "", storj.After, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "xb", storj.After, 0),
				result:  []string{"xbb", "xc"},
			}, {
				options: optionsRecursive("", "a/xbb", storj.After, 5),
				more:    true,
				result:  []string{"a/xc", "aa", "b", "b/ya", "b/yaa"},
			}, {
				options: options("a/", "xaa", storj.After, 2),
				more:    true,
				result:  []string{"xb", "xbb"},
			}, {
				options: options("", "", storj.Forward, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "`", storj.Forward, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "b", storj.Forward, 0),
				result:  []string{"b", "b/", "bb", "c"},
			}, {
				options: options("", "c", storj.Forward, 0),
				result:  []string{"c"},
			}, {
				options: options("", "ca", storj.Forward, 0),
				result:  []string{},
			}, {
				options: options("", "", storj.Forward, 1),
				more:    true,
				result:  []string{"a"},
			}, {
				options: options("", "`", storj.Forward, 1),
				more:    true,
				result:  []string{"a"},
			}, {
				options: options("", "aa", storj.Forward, 1),
				more:    true,
				result:  []string{"aa"},
			}, {
				options: options("", "c", storj.Forward, 1),
				result:  []string{"c"},
			}, {
				options: options("", "ca", storj.Forward, 1),
				result:  []string{},
			}, {
				options: options("", "", storj.Forward, 2),
				more:    true,
				result:  []string{"a", "a/"},
			}, {
				options: options("", "`", storj.Forward, 2),
				more:    true,
				result:  []string{"a", "a/"},
			}, {
				options: options("", "aa", storj.Forward, 2),
				more:    true,
				result:  []string{"aa", "b"},
			}, {
				options: options("", "bb", storj.Forward, 2),
				result:  []string{"bb", "c"},
			}, {
				options: options("", "c", storj.Forward, 2),
				result:  []string{"c"},
			}, {
				options: options("", "ca", storj.Forward, 2),
				result:  []string{},
			}, {
				options: optionsRecursive("", "", storj.Forward, 0),
				result:  []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			}, {
				options: options("a", "", storj.Forward, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "", storj.Forward, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "xb", storj.Forward, 0),
				result:  []string{"xb", "xbb", "xc"},
			}, {
				options: optionsRecursive("", "a/xbb", storj.Forward, 5),
				more:    true,
				result:  []string{"a/xbb", "a/xc", "aa", "b", "b/ya"},
			}, {
				options: options("a/", "xaa", storj.Forward, 2),
				more:    true,
				result:  []string{"xaa", "xb"},
			}, {
				options: options("", "", storj.Backward, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "`", storj.Backward, 0),
				result:  []string{},
			}, {
				options: options("", "b", storj.Backward, 0),
				result:  []string{"a", "a/", "aa", "b"},
			}, {
				options: options("", "c", storj.Backward, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "ca", storj.Backward, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "", storj.Backward, 1),
				more:    true,
				result:  []string{"c"},
			}, {
				options: options("", "`", storj.Backward, 1),
				result:  []string{},
			}, {
				options: options("", "aa", storj.Backward, 1),
				more:    true,
				result:  []string{"aa"},
			}, {
				options: options("", "c", storj.Backward, 1),
				more:    true,
				result:  []string{"c"},
			}, {
				options: options("", "ca", storj.Backward, 1),
				more:    true,
				result:  []string{"c"},
			}, {
				options: options("", "", storj.Backward, 2),
				more:    true,
				result:  []string{"bb", "c"},
			}, {
				options: options("", "`", storj.Backward, 2),
				result:  []string{},
			}, {
				options: options("", "a/", storj.Backward, 2),
				result:  []string{"a"},
			}, {
				options: options("", "bb", storj.Backward, 2),
				more:    true,
				result:  []string{"b/", "bb"},
			}, {
				options: options("", "c", storj.Backward, 2),
				more:    true,
				result:  []string{"bb", "c"},
			}, {
				options: options("", "ca", storj.Backward, 2),
				more:    true,
				result:  []string{"bb", "c"},
			}, {
				options: optionsRecursive("", "", storj.Backward, 0),
				result:  []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			}, {
				options: options("a", "", storj.Backward, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "", storj.Backward, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "xb", storj.Backward, 0),
				result:  []string{"xa", "xaa", "xb"},
			}, {
				options: optionsRecursive("", "b/yaa", storj.Backward, 5),
				more:    true,
				result:  []string{"a/xc", "aa", "b", "b/ya", "b/yaa"},
			}, {
				options: options("a/", "xbb", storj.Backward, 2),
				more:    true,
				result:  []string{"xb", "xbb"},
			}, {
				options: options("", "", storj.Before, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "`", storj.Before, 0),
				result:  []string{},
			}, {
				options: options("", "a", storj.Before, 0),
				result:  []string{},
			}, {
				options: options("", "b", storj.Before, 0),
				result:  []string{"a", "a/", "aa"},
			}, {
				options: options("", "c", storj.Before, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb"},
			}, {
				options: options("", "ca", storj.Before, 0),
				result:  []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			}, {
				options: options("", "", storj.Before, 1),
				more:    true,
				result:  []string{"c"},
			}, {
				options: options("", "`", storj.Before, 1),
				result:  []string{},
			}, {
				options: options("", "a/", storj.Before, 1),
				result:  []string{"a"},
			}, {
				options: options("", "c", storj.Before, 1),
				more:    true,
				result:  []string{"bb"},
			}, {
				options: options("", "ca", storj.Before, 1),
				more:    true,
				result:  []string{"c"},
			}, {
				options: options("", "", storj.Before, 2),
				more:    true,
				result:  []string{"bb", "c"},
			}, {
				options: options("", "`", storj.Before, 2),
				result:  []string{},
			}, {
				options: options("", "a/", storj.Before, 2),
				result:  []string{"a"},
			}, {
				options: options("", "bb", storj.Before, 2),
				more:    true,
				result:  []string{"b", "b/"},
			}, {
				options: options("", "c", storj.Before, 2),
				more:    true,
				result:  []string{"b/", "bb"},
			}, {
				options: options("", "ca", storj.Before, 2),
				more:    true,
				result:  []string{"bb", "c"},
			}, {
				options: optionsRecursive("", "", storj.Before, 0),
				result:  []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			}, {
				options: options("a", "", storj.Before, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "", storj.Before, 0),
				result:  []string{"xa", "xaa", "xb", "xbb", "xc"},
			}, {
				options: options("a/", "xb", storj.Before, 0),
				result:  []string{"xa", "xaa"},
			}, {
				options: optionsRecursive("", "b/yaa", storj.Before, 5),
				more:    true,
				result:  []string{"a/xbb", "a/xc", "aa", "b", "b/ya"},
			}, {
				options: options("a/", "xbb", storj.Before, 2),
				more:    true,
				result:  []string{"xaa", "xb"},
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			list, err := db.ListObjects(ctx, bucket.Name, tt.options)

			if assert.NoError(t, err, errTag) {
				assert.Equal(t, tt.more, list.More, errTag)
				for i, item := range list.Items {
					assert.Equal(t, tt.result[i], item.Path, errTag)
					assert.Equal(t, TestBucket, item.Bucket.Name, errTag)
					assert.Equal(t, storj.Unencrypted, item.Bucket.PathCipher, errTag)
				}
			}
		}
	})
}

func options(prefix, cursor string, direction storj.ListDirection, limit int) storj.ListOptions {
	return storj.ListOptions{
		Prefix:    prefix,
		Cursor:    cursor,
		Direction: direction,
		Limit:     limit,
	}
}

func optionsRecursive(prefix, cursor string, direction storj.ListDirection, limit int) storj.ListOptions {
	return storj.ListOptions{
		Prefix:    prefix,
		Cursor:    cursor,
		Direction: direction,
		Limit:     limit,
		Recursive: true,
	}
}
