// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package buckets

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/metabase"
)

var (
	// ErrBucketNotEmpty is returned when a caller attempts to change placement constraints.
	ErrBucketNotEmpty = errs.Class("bucket must be empty")
)

// NewService converts the provided db and metabase calls into a single DB interface.
func NewService(bucketsDB DB, metabase *metabase.DB) *Service {
	return &Service{
		DB:       bucketsDB,
		metabase: metabase,
	}
}

// Service encapsulates operations around buckets.
type Service struct {
	DB
	metabase *metabase.DB
}

// UpdateBucket overrides the default UpdateBucket behaviour by adding a check against MetabaseDB to ensure the bucket
// is empty before attempting to change the placement constraint of a bucket. If the placement constraint is not being
// changed, then this additional check is skipped.
func (buckets *Service) UpdateBucket(ctx context.Context, bucket Bucket) (Bucket, error) {
	current, err := buckets.GetBucket(ctx, []byte(bucket.Name), bucket.ProjectID)
	if err != nil {
		return Bucket{}, err
	}

	if current.Placement != bucket.Placement {
		ok, err := buckets.metabase.BucketEmpty(ctx, metabase.BucketEmpty{
			ProjectID:  bucket.ProjectID,
			BucketName: bucket.Name,
		})

		switch {
		case err != nil:
			return Bucket{}, err
		case !ok:
			return Bucket{}, ErrBucketNotEmpty.New("cannot modify placement constraint for non-empty bucket")
		}
	}

	return buckets.DB.UpdateBucket(ctx, bucket)
}
