// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"
	"testing"
	"flag"

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
)

const(
	TestAPIKey = "test-api-key"
	TestEncKey = "test-encryption-key"
	TestBucket = "testbucket"
)

func TestBuckets(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}

	defer ctx.Check(planet.Shutdown)

	planet.Start(context.Background())

	bucketStore, err := newBucketStore(planet)
	assert.NoError(t, err)

	buckets := NewBuckets(bucketStore)
	bucket, err := buckets.CreateBucket(ctx, TestBucket, nil)
	assert.NoError(t, err)
	assert.Equal(t, TestBucket, bucket.Name)

	bucketList, err := buckets.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
	assert.NoError(t, err)
	assert.False(t, bucketList.More)
	assert.Equal(t, 1, len(bucketList.Items))
	assert.Equal(t, TestBucket, bucketList.Items[0].Name)

	bucket, err = buckets.GetBucket(ctx, TestBucket)
	assert.NoError(t, err)
	assert.Equal(t, TestBucket, bucket.Name)

	err = buckets.DeleteBucket(ctx, TestBucket)
	assert.NoError(t, err)

	bucketList, err = buckets.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
	assert.NoError(t, err)
	assert.False(t, bucketList.More)
	assert.Equal(t, 0, len(bucketList.Items))
}

func newBucketStore(planet *testplanet.Planet) (buckets.Store, error) {
	// TODO(kaloyan): We should have a better way for configuring the Satellite's API Key
	flag.Set("pointer-db.auth.api-key", TestAPIKey)

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
