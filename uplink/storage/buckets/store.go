// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"

	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink/metainfo"
)

var mon = monkit.Package()

// Store for buckets
type Store interface {
	Create(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error)
	Get(ctx context.Context, bucketName string) (_ storj.Bucket, err error)
	Delete(ctx context.Context, bucketName string) (err error)
	List(ctx context.Context, listOpts storj.BucketListOptions) (_ storj.BucketList, err error)
}

// BucketStore is an object to interact with buckets
// via the metainfo client
type BucketStore struct {
	metainfoClient metainfo.Client
}

// NewStore creates a new bucket store
func NewStore(metainfoClient metainfo.Client) *BucketStore {
	return &BucketStore{
		metainfoClient: metainfoClient,
	}
}

// Create creates a bucket
func (store *BucketStore) Create(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	// uuid MarshalJSON implementation always returns err == nil
	partnerID, _ := bucket.PartnerID.MarshalJSON()
	return store.metainfoClient.CreateBucket(ctx, metainfo.CreateBucketParams{
		Name:                        []byte(bucket.Name),
		PathCipher:                  bucket.PathCipher,
		PartnerID:                   partnerID,
		DefaultSegmentsSize:         bucket.DefaultSegmentsSize,
		DefaultRedundancyScheme:     bucket.DefaultRedundancyScheme,
		DefaultEncryptionParameters: bucket.DefaultEncryptionParameters,
	})
}

// Get returns a bucket
func (store *BucketStore) Get(ctx context.Context, bucketName string) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return store.metainfoClient.GetBucket(ctx, metainfo.GetBucketParams{
		Name: []byte(bucketName),
	})
}

// Delete deletes a bucket
func (store *BucketStore) Delete(ctx context.Context, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return store.metainfoClient.DeleteBucket(ctx, metainfo.DeleteBucketParams{
		Name: []byte(bucketName),
	})
}

// List returns a list of buckets
func (store *BucketStore) List(ctx context.Context, listOpts storj.BucketListOptions) (_ storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	return store.metainfoClient.ListBuckets(ctx, metainfo.ListBucketsParams{
		ListOpts: listOpts,
	})
}
