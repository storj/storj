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
	bucket, err = store.metainfoClient.CreateBucket(ctx, bucket)
	if err != nil {
		return bucket, err
	}
	return bucket, err
}

// Get returns a bucket
func (store *BucketStore) Get(ctx context.Context, bucketName string) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	bucket, err := store.metainfoClient.GetBucket(ctx, bucketName)
	if err != nil {
		return storj.Bucket{}, err
	}
	return bucket, err
}

// Delete deletes a bucket
func (store *BucketStore) Delete(ctx context.Context, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.metainfoClient.DeleteBucket(ctx, bucketName)
	if err != nil {
		return err
	}
	return nil
}

// List returns a list of buckets
func (store *BucketStore) List(ctx context.Context, listOpts storj.BucketListOptions) (_ storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	bucketList, err := store.metainfoClient.ListBuckets(ctx, listOpts)
	if err != nil {
		return storj.BucketList{}, err
	}
	return bucketList, nil
}
