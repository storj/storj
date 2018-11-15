// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestGetObject(t *testing.T) {
	runTest(t, func(ctx context.Context, db *DB) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		store, err := db.buckets.GetObjectStore(ctx, bucket.Name)
		if !assert.NoError(t, err) {
			return
		}

		var exp time.Time
		_, err = store.Put(ctx, "test-file", bytes.NewReader(nil), objects.SerializableMeta{}, exp)
		if !assert.NoError(t, err) {
			return
		}

		_, err = db.GetObject(ctx, "", "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		// TODO: Currently returns ErrKeyNotFound instead of ErrEmptyKey
		// _, err = db.GetObject(ctx, bucket.Name, "")
		// assert.True(t, storage.ErrEmptyKey.Has(err))

		_, err = db.GetObject(ctx, "non-existing-bucket", "test-file")
		// TODO: Should return storj.ErrBucketNotFound
		assert.True(t, storage.ErrKeyNotFound.Has(err))

		_, err = db.GetObject(ctx, bucket.Name, "non-existing-file")
		assert.True(t, storage.ErrKeyNotFound.Has(err))

		object, err := db.GetObject(ctx, bucket.Name, "test-file")
		if assert.NoError(t, err) {
			assert.Equal(t, "test-file", object.Path)
		}
	})
}

func TestGetObjectStream(t *testing.T) {
	runTest(t, func(ctx context.Context, db *DB) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		store, err := db.buckets.GetObjectStore(ctx, bucket.Name)
		if !assert.NoError(t, err) {
			return
		}

		var exp time.Time
		_, err = store.Put(ctx, "empty-file", bytes.NewReader(nil), objects.SerializableMeta{}, exp)
		if !assert.NoError(t, err) {
			return
		}

		_, err = store.Put(ctx, "test-file", bytes.NewReader([]byte("test")), objects.SerializableMeta{}, exp)
		if !assert.NoError(t, err) {
			return
		}

		_, err = db.GetObjectStream(ctx, "", "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		// TODO: Currently returns ErrKeyNotFound instead of ErrEmptyKey
		// _, err = db.GetObjectStream(cstx, bucket.Name, "")
		// assert.True(t, storage.ErrEmptyKey.Has(err))

		_, err = db.GetObjectStream(ctx, "non-existing-bucket", "test-file")
		// TODO: Should return storj.ErrBucketNotFound
		assert.True(t, storage.ErrKeyNotFound.Has(err))

		_, err = db.GetObject(ctx, bucket.Name, "non-existing-file")
		assert.True(t, storage.ErrKeyNotFound.Has(err))

		stream, err := db.GetObjectStream(ctx, bucket.Name, "empty-file")
		if assert.NoError(t, err) {
			assertStream(ctx, t, stream, "empty-file", []byte(nil))
		}

		stream, err = db.GetObjectStream(ctx, bucket.Name, "test-file")
		if assert.NoError(t, err) {
			assertStream(ctx, t, stream, "test-file", []byte("test"))
		}
	})
}

func assertStream(ctx context.Context, t *testing.T, stream storj.ReadOnlyStream, path storj.Path, content []byte) bool {
	assert.Equal(t, path, stream.Info().Path)

	segments, more, err := stream.Segments(ctx, 0, 0)
	if !assert.NoError(t, err) {
		return false
	}

	assert.False(t, more)
	if !assert.Equal(t, 1, len(segments)) {
		return false

	}
	assert.EqualValues(t, 0, segments[0].Index)
	assert.EqualValues(t, len(content), segments[0].Size)

	// TODO: Currently Inline is always empty
	// assert.Equal(t, content, segments[0].Inline)

	return true
}

func TestDeleteObject(t *testing.T) {
	runTest(t, func(ctx context.Context, db *DB) {
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if !assert.NoError(t, err) {
			return
		}

		store, err := db.buckets.GetObjectStore(ctx, bucket.Name)
		if !assert.NoError(t, err) {
			return
		}

		var exp time.Time
		_, err = store.Put(ctx, "test-file", bytes.NewReader(nil), objects.SerializableMeta{}, exp)
		if !assert.NoError(t, err) {
			return
		}

		err = db.DeleteObject(ctx, "", "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		// TODO: Currently returns ErrKeyNotFound instead of ErrEmptyKey
		// err = db.DeleteObject(ctx, bucket.Name, "")
		// assert.True(t, storage.ErrEmptyKey.Has(err))

		_ = db.DeleteObject(ctx, "non-existing-bucket", "test-file")
		// TODO: Currently returns minio.BucketNotFound, should return storj.ErrBucketNotFound
		// assert.True(t, storj.ErrBucketNotFound.Has(err))

		err = db.DeleteObject(ctx, bucket.Name, "non-existing-file")
		assert.True(t, storage.ErrKeyNotFound.Has(err))

		err = db.DeleteObject(ctx, bucket.Name, "test-file")
		assert.NoError(t, err)
	})
}

func TestListObjectsEmpty(t *testing.T) {
	runTest(t, func(ctx context.Context, db *DB) {
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
	runTest(t, func(ctx context.Context, db *DB) {
		var exp time.Time
		bucket, err := db.CreateBucket(ctx, TestBucket, &storj.Bucket{PathCipher: storj.Unencrypted})
		if !assert.NoError(t, err) {
			return
		}

		store, err := db.buckets.GetObjectStore(ctx, bucket.Name)
		if !assert.NoError(t, err) {
			return
		}

		filePaths := []string{
			"a", "aa", "b", "bb", "c",
			"a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc",
			"b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc",
		}
		for _, path := range filePaths {
			_, err = store.Put(ctx, path, bytes.NewReader(nil), objects.SerializableMeta{}, exp)
			if !assert.NoError(t, err) {
				return
			}
		}

		otherBucket, err := db.CreateBucket(ctx, "otherbucket", nil)
		if !assert.NoError(t, err) {
			return
		}

		otherStore, err := db.buckets.GetObjectStore(ctx, otherBucket.Name)
		if !assert.NoError(t, err) {
			return
		}

		_, err = otherStore.Put(ctx, "file-in-other-bucket", bytes.NewReader(nil), objects.SerializableMeta{}, exp)
		if !assert.NoError(t, err) {
			return
		}

		for i, tt := range []struct {
			cursor    string
			dir       storj.ListDirection
			limit     int
			prefix    string
			recursive bool
			more      bool
			result    []string
		}{
			{
				dir:    storj.After,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				cursor: "`",
				dir:    storj.After,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				cursor: "b",
				dir:    storj.After,
				result: []string{"b/", "bb", "c"},
			},
			{
				cursor: "c",
				dir:    storj.After,
				result: []string{},
			},
			{
				cursor: "ca",
				dir:    storj.After,
				result: []string{},
			},
			{
				dir:    storj.After,
				limit:  1,
				more:   true,
				result: []string{"a"},
			},
			{
				cursor: "`",
				dir:    storj.After,
				limit:  1,
				more:   true,
				result: []string{"a"},
			},
			{
				cursor: "aa",
				dir:    storj.After,
				limit:  1,
				more:   true,
				result: []string{"b"},
			},
			{
				cursor: "c",
				dir:    storj.After,
				limit:  1,
				result: []string{},
			},
			{
				cursor: "ca",
				dir:    storj.After,
				limit:  1,
				result: []string{},
			},
			{
				dir:    storj.After,
				limit:  2,
				more:   true,
				result: []string{"a", "a/"},
			},
			{
				cursor: "`",
				dir:    storj.After,
				limit:  2,
				more:   true,
				result: []string{"a", "a/"},
			},
			{
				cursor: "aa",
				dir:    storj.After,
				limit:  2,
				more:   true,
				result: []string{"b", "b/"},
			},
			{
				cursor: "bb",
				dir:    storj.After,
				limit:  2,
				result: []string{"c"},
			},
			{
				cursor: "c",
				dir:    storj.After,
				limit:  2,
				result: []string{},
			},
			{
				cursor: "ca",
				dir:    storj.After,
				limit:  2,
				result: []string{},
			},
			{
				dir:       storj.After,
				recursive: true,
				result:    []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			},
			{
				dir:    storj.After,
				prefix: "a",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				dir:    storj.After,
				prefix: "a/",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				cursor: "xb",
				dir:    storj.After,
				prefix: "a/",
				result: []string{"xbb", "xc"},
			},
			{
				cursor:    "a/xbb",
				dir:       storj.After,
				limit:     5,
				more:      true,
				recursive: true,
				result:    []string{"a/xc", "aa", "b", "b/ya", "b/yaa"},
			},
			{
				cursor: "xaa",
				dir:    storj.After,
				limit:  2,
				more:   true,
				prefix: "a/",
				result: []string{"xb", "xbb"},
			},
			{
				dir:    storj.Forward,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				cursor: "`",
				dir:    storj.Forward,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				cursor: "b",
				dir:    storj.Forward,
				result: []string{"b", "b/", "bb", "c"},
			},
			{
				cursor: "c",
				dir:    storj.Forward,
				result: []string{"c"},
			},
			{
				cursor: "ca",
				dir:    storj.Forward,
				result: []string{},
			},
			{
				dir:    storj.Forward,
				limit:  1,
				more:   true,
				result: []string{"a"},
			},
			{
				cursor: "`",
				dir:    storj.Forward,
				limit:  1,
				more:   true,
				result: []string{"a"},
			},
			{
				cursor: "aa",
				dir:    storj.Forward,
				limit:  1,
				more:   true,
				result: []string{"aa"},
			},
			{
				cursor: "c",
				dir:    storj.Forward,
				limit:  1,
				result: []string{"c"},
			},
			{
				cursor: "ca",
				dir:    storj.Forward,
				limit:  1,
				result: []string{},
			},
			{
				dir:    storj.Forward,
				limit:  2,
				more:   true,
				result: []string{"a", "a/"},
			},
			{
				cursor: "`",
				dir:    storj.Forward,
				limit:  2,
				more:   true,
				result: []string{"a", "a/"},
			},
			{
				cursor: "aa",
				dir:    storj.Forward,
				limit:  2,
				more:   true,
				result: []string{"aa", "b"},
			},
			{
				cursor: "bb",
				dir:    storj.Forward,
				limit:  2,
				result: []string{"bb", "c"},
			},
			{
				cursor: "c",
				dir:    storj.Forward,
				limit:  2,
				result: []string{"c"},
			},
			{
				cursor: "ca",
				dir:    storj.Forward,
				limit:  2,
				result: []string{},
			},
			{
				dir:       storj.Forward,
				recursive: true,
				result:    []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			},
			{
				dir:    storj.Forward,
				prefix: "a",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				dir:    storj.Forward,
				prefix: "a/",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				cursor: "xb",
				dir:    storj.Forward,
				prefix: "a/",
				result: []string{"xb", "xbb", "xc"},
			},
			{
				cursor:    "a/xbb",
				dir:       storj.Forward,
				limit:     5,
				more:      true,
				recursive: true,
				result:    []string{"a/xbb", "a/xc", "aa", "b", "b/ya"},
			},
			{
				cursor: "xaa",
				dir:    storj.Forward,
				limit:  2,
				more:   true,
				prefix: "a/",
				result: []string{"xaa", "xb"},
			},
			{
				dir:    storj.Backward,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				cursor: "`",
				dir:    storj.Backward,
				result: []string{},
			},
			{
				cursor: "b",
				dir:    storj.Backward,
				result: []string{"a", "a/", "aa", "b"},
			},
			{
				cursor: "c",
				dir:    storj.Backward,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				cursor: "ca",
				dir:    storj.Backward,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				dir:    storj.Backward,
				limit:  1,
				more:   true,
				result: []string{"c"},
			},
			{
				cursor: "`",
				dir:    storj.Backward,
				limit:  1,
				result: []string{},
			},
			{
				cursor: "aa",
				dir:    storj.Backward,
				limit:  1,
				more:   true,
				result: []string{"aa"},
			},
			{
				cursor: "c",
				dir:    storj.Backward,
				limit:  1,
				more:   true,
				result: []string{"c"},
			},
			{
				cursor: "ca",
				dir:    storj.Backward,
				limit:  1,
				more:   true,
				result: []string{"c"},
			},
			{
				dir:    storj.Backward,
				limit:  2,
				more:   true,
				result: []string{"bb", "c"},
			},
			{
				cursor: "`",
				dir:    storj.Backward,
				limit:  2,
				result: []string{},
			},
			{
				cursor: "a/",
				dir:    storj.Backward,
				limit:  2,
				result: []string{"a"},
			},
			{
				cursor: "bb",
				dir:    storj.Backward,
				limit:  2,
				more:   true,
				result: []string{"b/", "bb"},
			},
			{
				cursor: "c",
				dir:    storj.Backward,
				limit:  2,
				more:   true,
				result: []string{"bb", "c"},
			},
			{
				cursor: "ca",
				dir:    storj.Backward,
				limit:  2,
				more:   true,
				result: []string{"bb", "c"},
			},
			{
				dir:       storj.Backward,
				recursive: true,
				result:    []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			},
			{
				dir:    storj.Backward,
				prefix: "a",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				dir:    storj.Backward,
				prefix: "a/",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				cursor: "xb",
				dir:    storj.Backward,
				prefix: "a/",
				result: []string{"xa", "xaa", "xb"},
			},
			{
				cursor:    "b/yaa",
				dir:       storj.Backward,
				limit:     5,
				more:      true,
				recursive: true,
				result:    []string{"a/xc", "aa", "b", "b/ya", "b/yaa"},
			},
			{
				cursor: "xbb",
				dir:    storj.Backward,
				limit:  2,
				more:   true,
				prefix: "a/",
				result: []string{"xb", "xbb"},
			},
			{
				dir:    storj.Before,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				cursor: "`",
				dir:    storj.Before,
				result: []string{},
			},
			{
				cursor: "a",
				dir:    storj.Before,
				result: []string{},
			},
			{
				cursor: "b",
				dir:    storj.Before,
				result: []string{"a", "a/", "aa"},
			},
			{
				cursor: "c",
				dir:    storj.Before,
				result: []string{"a", "a/", "aa", "b", "b/", "bb"},
			},
			{
				cursor: "ca",
				dir:    storj.Before,
				result: []string{"a", "a/", "aa", "b", "b/", "bb", "c"},
			},
			{
				dir:    storj.Before,
				limit:  1,
				more:   true,
				result: []string{"c"},
			},
			{
				cursor: "`",
				dir:    storj.Before,
				limit:  1,
				result: []string{},
			},
			{
				cursor: "a/",
				dir:    storj.Before,
				limit:  1,
				result: []string{"a"},
			},
			{
				cursor: "c",
				dir:    storj.Before,
				limit:  1,
				more:   true,
				result: []string{"bb"},
			},
			{
				cursor: "ca",
				dir:    storj.Before,
				limit:  1,
				more:   true,
				result: []string{"c"},
			},
			{
				dir:    storj.Before,
				limit:  2,
				more:   true,
				result: []string{"bb", "c"},
			},
			{
				cursor: "`",
				dir:    storj.Before,
				limit:  2,
				result: []string{},
			},
			{
				cursor: "a/",
				dir:    storj.Before,
				limit:  2,
				result: []string{"a"},
			},
			{
				cursor: "bb",
				dir:    storj.Before,
				limit:  2,
				more:   true,
				result: []string{"b", "b/"},
			},
			{
				cursor: "c",
				dir:    storj.Before,
				limit:  2,
				more:   true,
				result: []string{"b/", "bb"},
			},
			{
				cursor: "ca",
				dir:    storj.Before,
				limit:  2,
				more:   true,
				result: []string{"bb", "c"},
			},
			{
				dir:       storj.Before,
				recursive: true,
				result:    []string{"a", "a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc", "aa", "b", "b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc", "bb", "c"},
			},
			{
				dir:    storj.Before,
				prefix: "a",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				dir:    storj.Before,
				prefix: "a/",
				result: []string{"xa", "xaa", "xb", "xbb", "xc"},
			},
			{
				cursor: "xb",
				dir:    storj.Before,
				prefix: "a/",
				result: []string{"xa", "xaa"},
			},
			{
				cursor:    "b/yaa",
				dir:       storj.Before,
				limit:     5,
				more:      true,
				recursive: true,
				result:    []string{"a/xbb", "a/xc", "aa", "b", "b/ya"},
			},
			{
				cursor: "xbb",
				dir:    storj.Before,
				limit:  2,
				more:   true,
				prefix: "a/",
				result: []string{"xaa", "xb"},
			},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			list, err := db.ListObjects(ctx, bucket.Name, storj.ListOptions{
				Cursor:    tt.cursor,
				Direction: tt.dir,
				Limit:     tt.limit,
				Prefix:    tt.prefix,
				Recursive: tt.recursive,
			})

			if assert.NoError(t, err, errTag) {
				assert.Equal(t, tt.more, list.More, errTag)
				assert.Equal(t, tt.result, getObjectPaths(list), errTag)
			}
		}
	})
}

func getObjectPaths(list storj.ObjectList) []string {
	names := make([]string, len(list.Items))

	for i, item := range list.Items {
		names[i] = item.Path
	}

	return names
}
