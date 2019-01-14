// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"storj.io/storj/pkg/storj"
)

type DB interface {
	CreateBucket(ctx context.Context, info *storj.Bucket) (storj.Bucket, error)
	DeleteBucket(ctx context.Context, bucket string) error
	GetBucket(ctx context.Context, bucket string) (storj.Bucket, error)
	ListBuckets(ctx context.Context, options storj.BucketListOptions) (storj.BucketList, error)
}

type Service struct {
	db DB
}

func (s *Service) CreateBucket(ctx context.Context, info *storj.Bucket) (storj.Bucket, error) {
	// TODO: getAPIKey(ctx)
	return s.db.CreateBucket(ctx, info)
}

func (s *Service) DeleteBucket(ctx context.Context, bucket string) error {
	// TODO: getAPIKey(ctx)
	return s.db.DeleteBucket(ctx, bucket)
}

func (s *Service) GetBucket(ctx context.Context, bucket string) (storj.Bucket, error) {
	// TODO: getAPIKey(ctx)
	return s.db.GetBucket(ctx, bucket)
}

func (s *Service) ListBuckets(ctx context.Context, options storj.BucketListOptions) (storj.BucketList, error) {
	// TODO: getAPIKey(ctx)
	// TODO: check limit
	return s.db.ListBuckets(ctx, options)
}
