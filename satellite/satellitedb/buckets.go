// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/zeebo/errs"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type buckets struct {
	db dbx.Methods
}

// ListBuckets implements console.Buckets
func (buck *buckets) ListBuckets(ctx context.Context, projectID uuid.UUID) ([]console.Bucket, error) {
	buckets, err := buck.db.All_BucketInfo_By_ProjectId_OrderBy_Asc_Name(
		ctx,
		dbx.BucketInfo_ProjectId(projectID[:]),
	)

	if err != nil {
		return nil, err
	}

	var consoleBuckets []console.Bucket
	for _, bucket := range buckets {
		consoleBucket, bucketErr := fromDBXBucket(bucket)
		if err != nil {
			err = errs.Combine(err, bucketErr)
			continue
		}

		consoleBuckets = append(consoleBuckets, *consoleBucket)
	}

	if err != nil {
		return nil, err
	}

	return consoleBuckets, nil
}

// GetBucket implements console.Buckets
func (buck *buckets) GetBucket(ctx context.Context, name string) (*console.Bucket, error) {
	bucket, err := buck.db.Get_BucketInfo_By_Name(ctx, dbx.BucketInfo_Name(name))
	if err != nil {
		return nil, err
	}

	return fromDBXBucket(bucket)
}

// AttachBucket implements console.Buckets
func (buck *buckets) AttachBucket(ctx context.Context, name string, projectID uuid.UUID) (*console.Bucket, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	bucket, err := buck.db.Create_BucketInfo(
		ctx,
		dbx.BucketInfo_Id(id[:]),
		dbx.BucketInfo_ProjectId(projectID[:]),
		dbx.BucketInfo_Name(name),
	)

	if err != nil {
		return nil, err
	}

	return fromDBXBucket(bucket)
}

// DeattachBucket implements console.Buckets
func (buck *buckets) DeattachBucket(ctx context.Context, name string) error {
	_, err := buck.db.Delete_BucketInfo_By_Name(ctx, dbx.BucketInfo_Name(name))
	return err
}

// fromDBXBucket creates console.Bucket from dbx.Bucket
func fromDBXBucket(bucket *dbx.BucketInfo) (*console.Bucket, error) {
	id, err := bytesToUUID(bucket.Id)
	if err != nil {
		return nil, err
	}

	projectID, err := bytesToUUID(bucket.ProjectId)
	if err != nil {
		return nil, err
	}

	return &console.Bucket{
		ID:        id,
		ProjectID: projectID,
		Name:      bucket.Name,
		CreatedAt: bucket.CreatedAt,
	}, nil
}
