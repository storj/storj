// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo_test

import (
	"context"
	"fmt"
	"testing"

	"storj.io/storj/satellite/console"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

const (
	TestEncKey = "test-encryption-key"
	TestBucket = "test-bucket"
)

func TestBucketsBasic(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
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

func TestBucketsReadWrite(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
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

func TestErrNoBucket(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
		_, err := db.CreateBucket(ctx, "", nil)
		assert.True(t, storj.ErrNoBucket.Has(err))

		_, err = db.GetBucket(ctx, "")
		assert.True(t, storj.ErrNoBucket.Has(err))

		err = db.DeleteBucket(ctx, "")
		assert.True(t, storj.ErrNoBucket.Has(err))
	})
}

func TestBucketCreateCipher(t *testing.T) {
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
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
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
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
	runTest(t, func(t *testing.T, ctx context.Context, planet *testplanet.Planet, db *kvmetainfo.DB, streams streams.Store) {
		bucketNames := []string{"a", "aa", "b", "bb", "c"}

		for _, name := range bucketNames {
			_, err := db.CreateBucket(ctx, name, nil)
			require.NoError(t, err)
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

func runTest(t *testing.T, test func(*testing.T, context.Context, *testplanet.Planet, *kvmetainfo.DB, streams.Store)) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		db, streams, err := newMetainfoParts(planet)
		require.NoError(t, err)

		test(t, ctx, planet, db, streams)
	})
}

func newMetainfoParts(planet *testplanet.Planet) (*kvmetainfo.DB, streams.Store, error) {
	// TODO(kaloyan): We should have a better way for configuring the Satellite's API Key
	// add project to satisfy constraint
	project, err := planet.Satellites[0].DB.Console().Projects().Insert(context.Background(), &console.Project{
		Name: "testProject",
	})
	if err != nil {
		return nil, nil, err
	}

	apiKey, err := macaroon.NewAPIKey([]byte("testSecret"))
	if err != nil {
		return nil, nil, err
	}

	apiKeyInfo := console.APIKeyInfo{
		ProjectID: project.ID,
		Name:      "testKey",
		Secret:    []byte("testSecret"),
	}

	// add api key to db
	_, err = planet.Satellites[0].DB.Console().APIKeys().Create(context.Background(), apiKey.Head(), apiKeyInfo)
	if err != nil {
		return nil, nil, err
	}

	metainfo, err := planet.Uplinks[0].DialMetainfo(context.Background(), planet.Satellites[0], apiKey.Serialize())
	if err != nil {
		return nil, nil, err
	}

	ec := ecclient.NewClient(planet.Uplinks[0].Transport, 0)
	fc, err := infectious.NewFEC(2, 4)
	if err != nil {
		return nil, nil, err
	}

	rs, err := eestream.NewRedundancyStrategy(eestream.NewRSScheme(fc, 1*memory.KiB.Int()), 0, 0)
	if err != nil {
		return nil, nil, err
	}

	segments := segments.NewSegmentStore(metainfo, ec, rs, 8*memory.KiB.Int(), 8*memory.MiB.Int64())

	key := new(storj.Key)
	copy(key[:], TestEncKey)

	const stripesPerBlock = 2
	blockSize := stripesPerBlock * rs.StripeSize()
	inlineThreshold := 8 * memory.KiB.Int()
	streams, err := streams.NewStreamStore(segments, 64*memory.MiB.Int64(), key, blockSize, storj.AESGCM, inlineThreshold)
	if err != nil {
		return nil, nil, err
	}

	return kvmetainfo.New(metainfo, streams, segments, key, int32(blockSize), rs, 64*memory.MiB.Int64()), streams, nil
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
