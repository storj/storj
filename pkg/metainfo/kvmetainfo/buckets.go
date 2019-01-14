// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"storj.io/storj/pkg/storj"
)

// CreateBucket creates a new bucket with the specified information
func (db *DB) CreateBucket(ctx context.Context, bucket string, info *storj.Bucket) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	return db.metainfo.CreateBucket(ctx, bucket, info)
}

// DeleteBucket deletes bucket
func (db *DB) DeleteBucket(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.ErrNoBucket.New("")
	}

	return db.metainfo.DeleteBucket(ctx, bucket)
}

// GetBucket gets bucket information
func (db *DB) GetBucket(ctx context.Context, bucket string) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	return db.metainfo.GetBucket(ctx, bucket)
}

// ListBuckets lists buckets
func (db *DB) ListBuckets(ctx context.Context, options storj.BucketListOptions) (list storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	return db.metainfo.ListBuckets(ctx, options)
}
