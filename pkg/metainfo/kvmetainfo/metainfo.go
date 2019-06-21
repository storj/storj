// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kvmetainfo

import (
	"context"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/uplink/metainfo"
)

var mon = monkit.Package()

var errClass = errs.Class("kvmetainfo")

const defaultSegmentLimit = 8 // TODO

var _ storj.Metainfo = (*DB)(nil)

// DB implements metainfo database
type DB struct {
	project *Project

	metainfo metainfo.Client

	streams  streams.Store
	segments segments.Store

	rootKey *storj.Key
}

// New creates a new metainfo database
func New(metainfo metainfo.Client, streams streams.Store, segments segments.Store, rootKey *storj.Key, encryptedBlockSize int32, redundancy eestream.RedundancyStrategy, segmentsSize int64) *DB {
	return &DB{
		project:  NewProject(streams, encryptedBlockSize, redundancy, segmentsSize),
		metainfo: metainfo,
		streams:  streams,
		segments: segments,
		rootKey:  rootKey,
	}
}

// Limits returns limits for this metainfo database
func (db *DB) Limits() (storj.MetainfoLimits, error) {
	return storj.MetainfoLimits{
		ListLimit:                storage.LookupLimit,
		MinimumRemoteSegmentSize: memory.KiB.Int64(), // TODO: is this needed here?
		MaximumInlineSegmentSize: memory.MiB.Int64(),
	}, nil
}

// CreateBucket creates a new bucket with the specified information
func (db *DB) CreateBucket(ctx context.Context, bucketName string, info *storj.Bucket) (bucketInfo storj.Bucket, err error) {
	return db.project.CreateBucket(ctx, bucketName, info)
}

// DeleteBucket deletes bucket
func (db *DB) DeleteBucket(ctx context.Context, bucketName string) (err error) {
	return db.project.DeleteBucket(ctx, bucketName)
}

// GetBucket gets bucket information
func (db *DB) GetBucket(ctx context.Context, bucketName string) (bucketInfo storj.Bucket, err error) {
	return db.project.GetBucket(ctx, bucketName)
}

// ListBuckets lists buckets
func (db *DB) ListBuckets(ctx context.Context, options storj.BucketListOptions) (list storj.BucketList, err error) {
	return db.project.ListBuckets(ctx, options)
}
