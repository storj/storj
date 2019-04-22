// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storj"
)

// CreateBucket creates a new bucket with the specified information
func (db *Project) CreateBucket(ctx context.Context, bucketName string, info *storj.Bucket) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}
	if info == nil {
		info = &storj.Bucket{PathCipher: storj.AESGCM}
	}
	if info.EncryptionParameters.CipherSuite == storj.EncUnspecified {
		info.EncryptionParameters.CipherSuite = storj.EncAESGCM
	}
	if info.EncryptionParameters.BlockSize == 0 {
		info.EncryptionParameters.BlockSize = db.encryptedBlockSize
	}
	if info.RedundancyScheme.Algorithm == storj.InvalidRedundancyAlgorithm {
		info.RedundancyScheme.Algorithm = storj.ReedSolomon
	}
	if info.RedundancyScheme.RequiredShares == 0 {
		info.RedundancyScheme.RequiredShares = int16(db.redundancy.RequiredCount())
	}
	if info.RedundancyScheme.RepairShares == 0 {
		info.RedundancyScheme.RepairShares = int16(db.redundancy.RepairThreshold())
	}
	if info.RedundancyScheme.OptimalShares == 0 {
		info.RedundancyScheme.OptimalShares = int16(db.redundancy.OptimalThreshold())
	}
	if info.RedundancyScheme.TotalShares == 0 {
		info.RedundancyScheme.TotalShares = int16(db.redundancy.TotalCount())
	}
	if info.RedundancyScheme.ShareSize == 0 {
		info.RedundancyScheme.ShareSize = int32(db.redundancy.ErasureShareSize())
	}
	if info.SegmentsSize == 0 {
		info.SegmentsSize = db.segmentsSize
	}

	meta, err := db.buckets.Put(ctx, bucketName, buckets.Meta{
		PathEncryptionType: info.PathCipher,
		SegmentsSize:       info.SegmentsSize,
		RedundancyScheme:   info.RedundancyScheme,
		EncryptionScheme:   info.EncryptionParameters.ToEncryptionScheme(),
	})
	if err != nil {
		return storj.Bucket{}, err
	}

	return bucketFromMeta(bucketName, meta), nil
}

// DeleteBucket deletes bucket
func (db *Project) DeleteBucket(ctx context.Context, bucketName string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.ErrNoBucket.New("")
	}

	return db.buckets.Delete(ctx, bucketName)
}

// GetBucket gets bucket information
func (db *Project) GetBucket(ctx context.Context, bucketName string) (bucketInfo storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucketName == "" {
		return storj.Bucket{}, storj.ErrNoBucket.New("")
	}

	meta, err := db.buckets.Get(ctx, bucketName)
	if err != nil {
		return storj.Bucket{}, err
	}

	return bucketFromMeta(bucketName, meta), nil
}

// ListBuckets lists buckets
func (db *Project) ListBuckets(ctx context.Context, options storj.BucketListOptions) (list storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

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

	// TODO: remove this hack-fix of specifying the last key
	if options.Cursor == "" && (options.Direction == storj.Before || options.Direction == storj.Backward) {
		endBefore = "\x7f\x7f\x7f\x7f\x7f\x7f\x7f"
	}

	items, more, err := db.buckets.List(ctx, startAfter, endBefore, options.Limit)
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

func bucketFromMeta(bucketName string, meta buckets.Meta) storj.Bucket {
	return storj.Bucket{
		Name:                 bucketName,
		Created:              meta.Created,
		PathCipher:           meta.PathEncryptionType,
		SegmentsSize:         meta.SegmentsSize,
		RedundancyScheme:     meta.RedundancyScheme,
		EncryptionParameters: meta.EncryptionScheme.ToEncryptionParameters(),
	}
}
