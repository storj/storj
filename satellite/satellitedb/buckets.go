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

// ListBuckets returns bucket list of a given project
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

// GetBucket retrieves bucket info of bucket with given name
func (buck *buckets) GetBucket(ctx context.Context, name string) (*console.Bucket, error) {
	bucket, err := buck.db.Get_BucketInfo_By_Name(ctx, dbx.BucketInfo_Name(name))
	if err != nil {
		return nil, err
	}

	return fromDBXBucket(bucket)
}

// AttachBucket attaches a bucket to a project
func (buck *buckets) AttachBucket(ctx context.Context, name string, projectID uuid.UUID) (*console.Bucket, error) {
	bucket, err := buck.db.Create_BucketInfo(
		ctx,
		dbx.BucketInfo_ProjectId(projectID[:]),
		dbx.BucketInfo_Name(name),
	)

	if err != nil {
		return nil, err
	}

	return fromDBXBucket(bucket)
}

// DeattachBucket deletes bucket info for a bucket by name
func (buck *buckets) DeattachBucket(ctx context.Context, name string) error {
	_, err := buck.db.Delete_BucketInfo_By_Name(ctx, dbx.BucketInfo_Name(name))
	return err
}

// fromDBXBucket creates console.Bucket from dbx.Bucket
func fromDBXBucket(bucket *dbx.BucketInfo) (*console.Bucket, error) {
	projectID, err := bytesToUUID(bucket.ProjectId)
	if err != nil {
		return nil, err
	}

	return &console.Bucket{
		ProjectID: projectID,
		Name:      bucket.Name,
		CreatedAt: bucket.CreatedAt,
	}, nil
}
