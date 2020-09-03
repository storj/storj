// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo/metabase"
)

// BucketsDB is the interface for the database to interact with buckets
//
// architecture: Database
type BucketsDB interface {
	// Create creates a new bucket
	CreateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error)
	// Get returns an existing bucket
	GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket storj.Bucket, err error)
	// GetBucketID returns an existing bucket id.
	GetBucketID(ctx context.Context, bucket metabase.BucketLocation) (id uuid.UUID, err error)
	// UpdateBucket updates an existing bucket
	UpdateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error)
	// Delete deletes a bucket
	DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error)
	// List returns all buckets for a project
	ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList storj.BucketList, err error)
	// CountBuckets returns the number of buckets a project currently has
	CountBuckets(ctx context.Context, projectID uuid.UUID) (int, error)
}
