// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	TestAPIKey = "test-api-key"
	TestEncKey = "test-encryption-key"
	TestBucket = "testbucket"
)

func TestBucketsBasic(t *testing.T) {
	runTest(t, func(ctx context.Context, bucketStore buckets.Store) {
		buckets := NewBuckets(bucketStore)

		// Create new bucket
		bucket, err := buckets.CreateBucket(ctx, TestBucket, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
		}

		// Check that bucket list include the new bucket
		bucketList, err := buckets.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 1, len(bucketList.Items))
			assert.Equal(t, TestBucket, bucketList.Items[0].Name)
		}

		// Check that we can get the new bucket explicitly
		bucket, err = buckets.GetBucket(ctx, TestBucket)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
		}

		// Delete the bucket
		err = buckets.DeleteBucket(ctx, TestBucket)
		assert.NoError(t, err)

		// Check that the bucket list is empty
		bucketList, err = buckets.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 0, len(bucketList.Items))
		}

		// Check that the bucket cannot be get explicitly
		bucket, err = buckets.GetBucket(ctx, TestBucket)
		assert.True(t, storage.ErrKeyNotFound.Has(err))
	})
}

func TestBucketsReadNewWayWriteOldWay(t *testing.T) {
	runTest(t, func(ctx context.Context, bucketStore buckets.Store) {
		buckets := NewBuckets(bucketStore)

		// (Old API) Create new bucket
		_, err := bucketStore.Put(ctx, TestBucket)
		assert.NoError(t, err)

		// (New API) Check that bucket list include the new bucket
		bucketList, err := buckets.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 1, len(bucketList.Items))
			assert.Equal(t, TestBucket, bucketList.Items[0].Name)
		}

		// (New API) Check that we can get the new bucket explicitly
		bucket, err := buckets.GetBucket(ctx, TestBucket)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
		}

		// (Old API) Delete the bucket
		err = bucketStore.Delete(ctx, TestBucket)
		assert.NoError(t, err)

		// (New API) Check that the bucket list is empty
		bucketList, err = buckets.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
		if assert.NoError(t, err) {
			assert.False(t, bucketList.More)
			assert.Equal(t, 0, len(bucketList.Items))
		}

		// (New API) Check that the bucket cannot be get explicitly
		bucket, err = buckets.GetBucket(ctx, TestBucket)
		assert.True(t, storage.ErrKeyNotFound.Has(err))
	})
}

func TestBucketsReadOldWayWriteNewWay(t *testing.T) {
	runTest(t, func(ctx context.Context, bucketStore buckets.Store) {
		buckets := NewBuckets(bucketStore)

		// (New API) Create new bucket
		bucket, err := buckets.CreateBucket(ctx, TestBucket, nil)
		if assert.NoError(t, err) {
			assert.Equal(t, TestBucket, bucket.Name)
		}

		// (Old API) Check that bucket list include the new bucket
		items, more, err := bucketStore.List(ctx, "", "", 0)
		if assert.NoError(t, err) {
			assert.False(t, more)
			assert.Equal(t, 1, len(items))
			assert.Equal(t, TestBucket, items[0].Bucket)
		}

		// (Old API) Check that we can get the new bucket explicitly
		_, err = bucketStore.Get(ctx, TestBucket)
		assert.NoError(t, err)

		// (New API) Delete the bucket
		err = buckets.DeleteBucket(ctx, TestBucket)
		assert.NoError(t, err)

		// (Old API) Check that the bucket list is empty
		items, more, err = bucketStore.List(ctx, "", "", 0)
		if assert.NoError(t, err) {
			assert.False(t, more)
			assert.Equal(t, 0, len(items))
		}

		// (Old API) Check that the bucket cannot be get explicitly
		_, err = bucketStore.Get(ctx, TestBucket)
		assert.True(t, storage.ErrKeyNotFound.Has(err))
	})
}

func TestErrNoBucket(t *testing.T) {
	runTest(t, func(ctx context.Context, bucketStore buckets.Store) {
		buckets := NewBuckets(bucketStore)

		_, err := buckets.CreateBucket(ctx, "", nil)
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = buckets.GetBucket(ctx, "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		err = buckets.DeleteBucket(ctx, "")
		assert.True(t, storj.ErrNoBucket.Has(err))
	})
}

func TestListBucketsEmpty(t *testing.T) {
	runTest(t, func(ctx context.Context, bucketStore buckets.Store) {
		buckets := NewBuckets(bucketStore)

		_, err := buckets.ListBuckets(ctx, storj.BucketListOptions{})
		assert.EqualError(t, err, "kvmetainfo: invalid direction 0")

		for _, direction := range []storj.ListDirection{
			storj.Before,
			storj.Backward,
			storj.Forward,
			storj.After,
		} {
			bucketList, err := buckets.ListBuckets(ctx, storj.BucketListOptions{Direction: direction})
			if assert.NoError(t, err) {
				assert.False(t, bucketList.More)
				assert.Equal(t, 0, len(bucketList.Items))
			}
		}
	})
}

func runTest(t *testing.T, test func(context.Context, buckets.Store)) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(1, 4, 1)
	if !assert.NoError(t, err) {
		return
	}

	defer ctx.Check(planet.Shutdown)

	planet.Start(context.Background())

	bucketStore, err := newBucketStore(planet)
	if !assert.NoError(t, err) {
		return
	}

	test(ctx, bucketStore)
}

func newBucketStore(planet *testplanet.Planet) (buckets.Store, error) {
	// TODO(kaloyan): We should have a better way for configuring the Satellite's API Key
	err := flag.Set("pointer-db.auth.api-key", TestAPIKey)
	if err != nil {
		return nil, err
	}

	oc, err := overlay.NewOverlayClient(planet.Uplinks[0].Identity, planet.Uplinks[0].Addr())
	if err != nil {
		return nil, err
	}

	pdb, err := planet.Uplinks[0].DialPointerDB(planet.Satellites[0], TestAPIKey)
	if err != nil {
		return nil, err
	}

	ec := ecclient.NewClient(planet.Uplinks[0].Identity, 0)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		return nil, err
	}

	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, int(1*memory.KB)), 3, 4)
	if err != nil {
		return nil, err
	}

	segments := segments.NewSegmentStore(oc, ec, pdb, rs, int(8*memory.KB))

	key := new(storj.Key)
	copy(key[:], TestEncKey)

	stream, err := streams.NewStreamStore(segments, int64(64*memory.MB), key, int(1*memory.KB), storj.AESGCM)
	if err != nil {
		return nil, err
	}

	obj := objects.NewStore(stream)

	return buckets.NewStore(obj), nil
}
