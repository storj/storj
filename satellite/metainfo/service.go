// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

var (
	// ErrBucketNotEmpty is returned when bucket is required to be empty for an operation.
	ErrBucketNotEmpty = errs.Class("bucket not empty")
)

// Service provides the metainfo service dependencies.
//
// architecture: Service
type Service struct {
	logger     *zap.Logger
	bucketsDB  BucketsDB
	metabaseDB MetabaseDB
}

// NewService creates new metainfo service.
func NewService(logger *zap.Logger, bucketsDB BucketsDB, metabaseDB MetabaseDB) *Service {
	return &Service{
		logger:     logger,
		bucketsDB:  bucketsDB,
		metabaseDB: metabaseDB,
	}
}

// CreateBucket creates a new bucket in the buckets db.
func (s *Service) CreateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.CreateBucket(ctx, bucket)
}

// HasBucket returns if a bucket exists.
func (s *Service) HasBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (ok bool, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.HasBucket(ctx, bucketName, projectID)
}

// GetBucket returns an existing bucket in the buckets db.
func (s *Service) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.GetBucket(ctx, bucketName, projectID)
}

// UpdateBucket returns an updated bucket in the buckets db.
func (s *Service) UpdateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.UpdateBucket(ctx, bucket)
}

// DeleteBucket deletes a bucket from the bucekts db.
func (s *Service) DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	empty, err := s.IsBucketEmpty(ctx, projectID, bucketName)
	if err != nil {
		return err
	}
	if !empty {
		return ErrBucketNotEmpty.New("")
	}

	return s.bucketsDB.DeleteBucket(ctx, bucketName, projectID)
}

// IsBucketEmpty returns whether bucket is empty.
func (s *Service) IsBucketEmpty(ctx context.Context, projectID uuid.UUID, bucketName []byte) (bool, error) {
	empty, err := s.metabaseDB.BucketEmpty(ctx, metabase.BucketEmpty{
		ProjectID:  projectID,
		BucketName: string(bucketName),
	})
	return empty, Error.Wrap(err)
}

// ListBuckets returns a list of buckets for a project.
func (s *Service) ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.ListBuckets(ctx, projectID, listOpts, allowedBuckets)
}

// CountBuckets returns the number of buckets a project currently has.
func (s *Service) CountBuckets(ctx context.Context, projectID uuid.UUID) (count int, err error) {
	defer mon.Task()(&ctx)(&err)
	return s.bucketsDB.CountBuckets(ctx, projectID)
}
