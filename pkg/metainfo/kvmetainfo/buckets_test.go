// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo_test

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/storage/buckets"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

const (
	TestAPIKey = "test-api-key"
	TestEncKey = "test-encryption-key"
	TestBucket = "test-bucket"
)

func TestBucketsBasic(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		// Create new bucket
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
		}

		// Check that bucket list include the new bucket
		bucketList, err := db.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 1, len(bucketList.Items))
			assert.Equal(t, TestBucket, bucketList.Items[0].Name)
		}

		// Check that we can get the new bucket explicitly
		bucket, err = db.GetBucket(ctx, TestBucket)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
			assert.Equal(t, storj.AESGCM, bucket.PathCipher)
		}

		// Delete the bucket
		err = db.DeleteBucket(ctx, TestBucket)
		assert.NoError(t, err)

		// Check that the bucket list is empty
		bucketList, err = db.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 0, len(bucketList.Items))
		}

		// Check that the bucket cannot be get explicitly
		bucket, err = db.GetBucket(ctx, TestBucket)
		assert.True(t, storj.ErrBucketNotFound.Has(err))
	})
}

func TestBucketsReadNewWayWriteOldWay(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		// (Old API) Create new bucket
		_, err := buckets.Put(ctx, TestBucket, storj.AESGCM)
		assert.NoError(t, err)

		// (New API) Check that bucket list include the new bucket
		bucketList, err := db.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 1, len(bucketList.Items))
			assert.Equal(t, TestBucket, bucketList.Items[0].Name)
		}

		// (New API) Check that we can get the new bucket explicitly
		bucket, err := db.GetBucket(ctx, TestBucket)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
			assert.Equal(t, storj.AESGCM, bucket.PathCipher)
		}

		// (Old API) Delete the bucket
		err = buckets.Delete(ctx, TestBucket)
		assert.NoError(t, err)

		// (New API) Check that the bucket list is empty
		bucketList, err = db.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 0, len(bucketList.Items))
		}

		// (New API) Check that the bucket cannot be get explicitly
		bucket, err = db.GetBucket(ctx, TestBucket)
		assert.True(t, storj.ErrBucketNotFound.Has(err))
	})
}

func TestBucketsReadOldWayWriteNewWay(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		// (New API) Create new bucket
		bucket, err := db.CreateBucket(ctx, TestBucket, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
		}

		// (Old API) Check that bucket list include the new bucket
		items, more, err := buckets.List(ctx, "", "", 0)
		if assert.NoError(t, err) {
			assert.False(t, more)
			assert.Equal(t, 1, len(items))
			assert.Equal(t, TestBucket, items[0].Bucket)
		}

		// (Old API) Check that we can get the new bucket explicitly
		meta, err := buckets.Get(ctx, TestBucket)
		if assert.NoError(t, err) {
			assert.Equal(t, storj.AESGCM, meta.PathEncryptionType)
		}

		// (New API) Delete the bucket
		err = db.DeleteBucket(ctx, TestBucket)
		assert.NoError(t, err)

		// (Old API) Check that the bucket list is empty
		items, more, err = buckets.List(ctx, "", "", 0)
		if assert.NoError(t, err) {
			assert.False(t, more)
			assert.Equal(t, 0, len(items))
		}

		// (Old API) Check that the bucket cannot be get explicitly
		_, err = buckets.Get(ctx, TestBucket)
		assert.True(t, storj.ErrBucketNotFound.Has(err))
	})
}

func TestErrNoBucket(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		_, err := db.CreateBucket(ctx, "", nil)
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.GetBucket(ctx, "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		err = db.DeleteBucket(ctx, "")
		assert.True(t, storj.ErrNoBucket.Has(err))
	})
}

func TestBucketCreateCipher(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		forAllCiphers(func(cipher storj.Cipher) {
			bucket, err := db.CreateBucket(ctx, "test", &storj.Bucket{PathCipher: cipher})
			if assert.NoError(t, err) {
				assert.Equal(t, cipher, bucket.PathCipher)
			}

			bucket, err = db.GetBucket(ctx, "test")
			if assert.NoError(t, err) {
				assert.Equal(t, cipher, bucket.PathCipher)
			}

			err = db.DeleteBucket(ctx, "test")
			assert.NoError(t, err)
		})
	})
}

func TestListBucketsEmpty(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		_, err := db.ListBuckets(ctx, storj.BucketListOptions{})
		assert.EqualError(t, err, "kvmetainfo: invalid direction 0")

		for _, direction := range []storj.ListDirection{
			storj.Before,
			storj.Backward,
			storj.Forward,
			storj.After,
		} {
			bucketList, err := db.ListBuckets(ctx, storj.BucketListOptions{Direction: direction})
			if assert.NoError(t, err) {
				assert.False(t, bucketList.More)
				assert.Equal(t, 0, len(bucketList.Items))
			}
		}
	})
}

func TestListBuckets(t *testing.T) {
	runTest(t, func(ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, buckets buckets.Store, streams streams.Store) {
		bucketNames := []string{"a", "aa", "b", "bb", "c"}

		for _, name := range bucketNames {
			_, err := db.CreateBucket(ctx, name, nil)
			if !assert.NoError(t, err) {
				return
			}
		}

		for i, tt := range []struct {
			cursor string
			dir    storj.ListDirection
			limit  int
			more   bool
			result []string
		}{
			{cursor: "", dir: storj.After, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.After, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "b", dir: storj.After, limit: 0, more: false, result: []string{"bb", "c"}},
			{cursor: "c", dir: storj.After, limit: 0, more: false, result: []string{}},
			{cursor: "ca", dir: storj.After, limit: 0, more: false, result: []string{}},
			{cursor: "", dir: storj.After, limit: 1, more: true, result: []string{"a"}},
			{cursor: "`", dir: storj.After, limit: 1, more: true, result: []string{"a"}},
			{cursor: "aa", dir: storj.After, limit: 1, more: true, result: []string{"b"}},
			{cursor: "c", dir: storj.After, limit: 1, more: false, result: []string{}},
			{cursor: "ca", dir: storj.After, limit: 1, more: false, result: []string{}},
			{cursor: "", dir: storj.After, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "`", dir: storj.After, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "aa", dir: storj.After, limit: 2, more: true, result: []string{"b", "bb"}},
			{cursor: "bb", dir: storj.After, limit: 2, more: false, result: []string{"c"}},
			{cursor: "c", dir: storj.After, limit: 2, more: false, result: []string{}},
			{cursor: "ca", dir: storj.After, limit: 2, more: false, result: []string{}},
			{cursor: "", dir: storj.Forward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.Forward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "b", dir: storj.Forward, limit: 0, more: false, result: []string{"b", "bb", "c"}},
			{cursor: "c", dir: storj.Forward, limit: 0, more: false, result: []string{"c"}},
			{cursor: "ca", dir: storj.Forward, limit: 0, more: false, result: []string{}},
			{cursor: "", dir: storj.Forward, limit: 1, more: true, result: []string{"a"}},
			{cursor: "`", dir: storj.Forward, limit: 1, more: true, result: []string{"a"}},
			{cursor: "aa", dir: storj.Forward, limit: 1, more: true, result: []string{"aa"}},
			{cursor: "c", dir: storj.Forward, limit: 1, more: false, result: []string{"c"}},
			{cursor: "ca", dir: storj.Forward, limit: 1, more: false, result: []string{}},
			{cursor: "", dir: storj.Forward, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "`", dir: storj.Forward, limit: 2, more: true, result: []string{"a", "aa"}},
			{cursor: "aa", dir: storj.Forward, limit: 2, more: true, result: []string{"aa", "b"}},
			{cursor: "bb", dir: storj.Forward, limit: 2, more: false, result: []string{"bb", "c"}},
			{cursor: "c", dir: storj.Forward, limit: 2, more: false, result: []string{"c"}},
			{cursor: "ca", dir: storj.Forward, limit: 2, more: false, result: []string{}},
			{cursor: "", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.Backward, limit: 0, more: false, result: []string{}},
			{cursor: "b", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b"}},
			{cursor: "c", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "ca", dir: storj.Backward, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "", dir: storj.Backward, limit: 1, more: true, result: []string{"c"}},
			{cursor: "`", dir: storj.Backward, limit: 1, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Backward, limit: 1, more: true, result: []string{"aa"}},
			{cursor: "c", dir: storj.Backward, limit: 1, more: true, result: []string{"c"}},
			{cursor: "ca", dir: storj.Backward, limit: 1, more: true, result: []string{"c"}},
			{cursor: "", dir: storj.Backward, limit: 2, more: true, result: []string{"bb", "c"}},
			{cursor: "`", dir: storj.Backward, limit: 2, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Backward, limit: 2, more: false, result: []string{"a", "aa"}},
			{cursor: "bb", dir: storj.Backward, limit: 2, more: true, result: []string{"b", "bb"}},
			{cursor: "c", dir: storj.Backward, limit: 2, more: true, result: []string{"bb", "c"}},
			{cursor: "ca", dir: storj.Backward, limit: 2, more: true, result: []string{"bb", "c"}},
			{cursor: "", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "`", dir: storj.Before, limit: 0, more: false, result: []string{}},
			{cursor: "b", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa"}},
			{cursor: "c", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa", "b", "bb"}},
			{cursor: "ca", dir: storj.Before, limit: 0, more: false, result: []string{"a", "aa", "b", "bb", "c"}},
			{cursor: "", dir: storj.Before, limit: 1, more: true, result: []string{"c"}},
			{cursor: "`", dir: storj.Before, limit: 1, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Before, limit: 1, more: false, result: []string{"a"}},
			{cursor: "c", dir: storj.Before, limit: 1, more: true, result: []string{"bb"}},
			{cursor: "ca", dir: storj.Before, limit: 1, more: true, result: []string{"c"}},
			{cursor: "", dir: storj.Before, limit: 2, more: true, result: []string{"bb", "c"}},
			{cursor: "`", dir: storj.Before, limit: 2, more: false, result: []string{}},
			{cursor: "aa", dir: storj.Before, limit: 2, more: false, result: []string{"a"}},
			{cursor: "bb", dir: storj.Before, limit: 2, more: true, result: []string{"aa", "b"}},
			{cursor: "c", dir: storj.Before, limit: 2, more: true, result: []string{"b", "bb"}},
			{cursor: "ca", dir: storj.Before, limit: 2, more: true, result: []string{"bb", "c"}},
		} {
			errTag := fmt.Sprintf("%d. %+v", i, tt)

			bucketList, err := db.ListBuckets(ctx, storj.BucketListOptions{
				Cursor:    tt.cursor,
				Direction: tt.dir,
				Limit:     tt.limit,
			})

			if assert.NoError(t, err, errTag) {
				assert.Equal(t, tt.more, bucketList.More, errTag)
				assert.Equal(t, tt.result, getBucketNames(bucketList), errTag)
			}
		}
	})
}

func getBucketNames(bucketList storj.BucketList) []string {
	names := make([]string, len(bucketList.Items))

	for i, item := range bucketList.Items {
		names[i] = item.Name
	}

	return names
}

func runTest(t *testing.T, test func(context.Context, *testplanet.Planet, *kvmetainfo.DB, buckets.Store, streams.Store)) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 4, 1)
	if !assert.NoError(t, err) {
		return
	}

	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	db, buckets, streams, err := newMetainfoParts(planet)
	if !assert.NoError(t, err) {
		return
	}

	test(ctx, planet, db, buckets, streams)
}

func newMetainfoParts(planet *testplanet.Planet) (*kvmetainfo.DB, buckets.Store, streams.Store, error) {
	// TODO(kaloyan): We should have a better way for configuring the Satellite's API Key
	err := flag.Set("pointer-db.auth.api-key", TestAPIKey)
	if err != nil {
		return nil, nil, nil, err
	}

	oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
	if err != nil {
		return nil, nil, nil, err
	}

	pdb, err := planet.Uplinks[0].DialPointerDB(planet.Satellites[0], TestAPIKey)
	if err != nil {
		return nil, nil, nil, err
	}

	ec := ecclient.NewClient(planet.Uplinks[0].Identity, 0)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		return nil, nil, nil, err
	}

	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, 1*memory.KB.Int()), 3, 4)
	if err != nil {
		return nil, nil, nil, err
	}

	segments := segments.NewSegmentStore(oc, ec, pdb, rs, 8*memory.KB.Int())

	key := new(storj.Key)
	copy(key[:], TestEncKey)

	streams, err := streams.NewStreamStore(segments, 64*memory.MB.Int64(), key, 1*memory.KB.Int(), storj.AESGCM)
	if err != nil {
		return nil, nil, nil, err
	}

	buckets := buckets.NewStore(streams)

	return kvmetainfo.New(buckets, streams, segments, pdb, key), buckets, streams, nil
}

func forAllCiphers(test func(cipher storj.Cipher)) {
	for _, cipher := range []storj.Cipher{
		storj.Unencrypted,
		storj.AESGCM,
		storj.SecretBox,
	} {
		test(cipher)
	}
}
