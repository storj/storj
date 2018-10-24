// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storj"
)

var errClass = errs.Class("kvmetainfo")

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
	switch options.Direction {
	case storj.Before: // Before lists backwards from cursor, without cursor
		endBefore = options.Cursor
	case storj.Backward: // Backward lists backwards from cursor, including cursor
		endBefore = options.Cursor + "\x00"
	case storj.Forward: // Forward lists forwards from cursor, including cursor
		startAfter = firstToStartAfter(options.Cursor)
	case storj.After: // After lists forwards from cursor, without cursor
		startAfter = options.Cursor
	default:
		return storj.BucketList{}, errClass.New("invalid direction %d", options.Direction)
	}

	items, more, err := db.store.List(ctx, startAfter, endBefore, options.Limit)
	if err != nil {
		return storj.BucketList{}, err
	}

	list := storj.BucketList{More: more}

	for _, item := range items {
		list.Buckets = append(list.Buckets,
			bucketFromMeta(item.Bucket, item.Meta))
	}

	return list, nil
}

func bucketFromMeta(bucket string, meta buckets.Meta) storj.Bucket {
	return storj.Bucket{
		Name:    bucket,
		Created: meta.Created,
	}
}
