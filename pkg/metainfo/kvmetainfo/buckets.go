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
		return storj.Bucket{}, storj.NoBucketError.New("")
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
		return storj.NoBucketError.New("")
	}

	return db.store.Delete(ctx, bucket)
}

// GetBucket gets bucket information
func (db *Buckets) GetBucket(ctx context.Context, bucket string) (storj.Bucket, error) {
	if bucket == "" {
		return storj.Bucket{}, storj.NoBucketError.New("")
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
	switch options.Direction {
	case storj.Before:
		// before lists backwards from cursor, without cursor
		endBefore = options.Cursor
	case storj.Backward:
		// backward lists backwards from cursor, including cursor
		endBefore = keyAfter(options.Cursor)
	case storj.Forward:
		// forward lists forwards from cursor, including cursor
		startAfter = keyBefore(options.Cursor)
	case storj.After:
		// after lists forwards from cursor, without cursor
		startAfter = options.Cursor
	default:
		return storj.BucketList{}, errClass.New("invalid direction %d", options.Direction)
	}

	items, more, err := db.store.List(ctx, startAfter, endBefore, options.Limit)
	if err != nil {
		return storj.BucketList{}, err
	}

	list := storj.BucketList{
		More:  more,
		Items: make([]storj.Bucket, 0, len(items)),
	}

	for _, item := range items {
		list.Items = append(list.Items, bucketFromMeta(item.Bucket, item.Meta))
	}

	return list, nil
}

func bucketFromMeta(bucket string, meta buckets.Meta) storj.Bucket {
	return storj.Bucket{
		Name:    bucket,
		Created: meta.Created,
	}
}
