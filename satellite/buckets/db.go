// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/storj"
)

// DB is the interface for the database to interact with buckets
type DB interface {
	// Create creates a new bucket
	CreateBucket(ctx context.Context, bucket storj.Bucket) (err error)
	// Get returns an existing bucket
	GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket storj.Bucket, err error)
	// Delete deletes a bucket
	DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error)
	// List returns all buckets for a project
	ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions) (buckets []storj.Bucket, err error)
}
