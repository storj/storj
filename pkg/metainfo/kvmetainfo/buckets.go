// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storj"
)

// CreateBucket creates a new bucket with the specified information
func (db *DB) CreateBucket(ctx context.Context, bucket string, info *storj.Bucket) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	meta, err := db.buckets.Put(ctx, bucket, getPathCipher(info))
	if err != nil {
		return storj.Bucket{}, err
	}

	return bucketFromMeta(bucket, meta), nil
}

// DeleteBucket deletes bucket
func (db *DB) DeleteBucket(ctx context.Context, bucket string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.ErrNoBucket.New("")
	}

	return db.buckets.Delete(ctx, bucket)
}

// GetBucket gets bucket information
func (db *DB) GetBucket(ctx context.Context, bucket string) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	meta, err := db.buckets.Get(ctx, bucket)
	if err != nil {
		return storj.Bucket{}, err
	}

	return bucketFromMeta(bucket, meta), nil
}

// ListBuckets lists buckets
func (db *DB) ListBuckets(ctx context.Context, options storj.BucketListOptions) (list storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	items, more, err := db.buckets.List(ctx, options.Cursor, "", options.Limit)
	if err != nil {
		return storj.BucketList{}, err
	}

	list = storj.BucketList{
		More:  more,
		Items: make([]storj.Bucket, 0, len(items)),
	}

	for _, item := range items {
		list.Items = append(list.Items, bucketFromMeta(item.Bucket, item.Meta))
	}

	return list, nil
}

func getPathCipher(info *storj.Bucket) storj.Cipher {
	if info == nil {
		return storj.AESGCM
	}
	return info.PathCipher
}

func bucketFromMeta(bucket string, meta buckets.Meta) storj.Bucket {
	return storj.Bucket{
		Name:       bucket,
		Created:    meta.Created,
		PathCipher: meta.PathEncryptionType,
	}
}
