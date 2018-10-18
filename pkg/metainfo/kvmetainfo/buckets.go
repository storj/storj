// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storj"
)

// Buckets implements storj.Metainfo bucket handling
type Buckets struct {
	store buckets.Store
}

// NewBuckets creates Buckets
func NewBuckets(store buckets.Store) *Buckets { return &Buckets{store} }

// CreateBucket creates a new bucket with the specified information
func (db *Buckets) CreateBucket(ctx context.Context, bucket string, info *storj.Bucket) (storj.Bucket, error) {
	if bucket == "" {
		return storj.Bucket{}, buckets.NoBucketError.New("")
	}

	meta, err := db.store.Put(ctx, bucket)
	if err != nil {
		return storj.Bucket{}, err
	}

	return bucketFromMeta(bucket, meta), nil
}

// DeleteBucket deletes bucket
func (db *Buckets) DeleteBucket(ctx context.Context, bucket string) error {
	if bucket == "" {
		return buckets.NoBucketError.New("")
	}

	return db.store.Delete(ctx, bucket)
}

// GetBucket gets bucket information
func (db *Buckets) GetBucket(ctx context.Context, bucket string) (storj.Bucket, error) {
	if bucket == "" {
		return storj.Bucket{}, buckets.NoBucketError.New("")
	}

	meta, err := db.store.Get(ctx, bucket)
	if err != nil {
		return storj.Bucket{}, err
	}

	return bucketFromMeta(bucket, meta), nil
}

// ListBuckets lists buckets
func (db *Buckets) ListBuckets(ctx context.Context, options storj.BucketListOptions) (storj.BucketList, error) {
	var startAfter, endBefore string
	if !options.Reverse {
		startAfter = firstToStartAfter(options.First)
	} else {
		endBefore = options.First + "\x00"
	}

	items, more, err := db.store.List(ctx, startAfter, endBefore, options.List)
	if err != nil {
		return storj.BucketList{}, err
	}

	list := storj.BucketList{
		NextFirst: "",
		More:      more,
	}

	for _, item := range items {
		list.Buckets = append(list.Buckets,
			bucketFromMeta(item.Bucket, item.Meta))
	}

	if len(list.Buckets) > 0 && more {
		if !options.Reverse {
			list.NextFirst = list.Buckets[len(list.Buckets)-1].Name + "\x00"
		} else {
			list.NextFirst = firstToStartAfter(list.Buckets[0].Name)
		}
	}

	return list, nil
}

func bucketFromMeta(bucket string, meta buckets.Meta) storj.Bucket {
	return storj.Bucket{
		Name:    bucket,
		Created: meta.Created,
	}
}
