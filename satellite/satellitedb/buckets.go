// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/buckets"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bucketsDB struct {
	db dbx.Methods
}

// CreateBucket creates a new bucket
func (db *bucketsDB) CreateBucket(ctx context.Context, bucket storj.Bucket) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Create_Bucket(ctx,
		dbx.Bucket_Id(bucket.ID[:]),
		dbx.Bucket_ProjectId(bucket.ProjectID[:]),
		dbx.Bucket_Name([]byte(bucket.Name)),
		dbx.Bucket_PathCipher(int(bucket.PathCipher)),
		dbx.Bucket_DefaultSegmentSize(int(bucket.SegmentsSize)),
		dbx.Bucket_DefaultEncryptionCipherSuite(int(bucket.EncryptionParameters.CipherSuite)),
		dbx.Bucket_DefaultEncryptionBlockSize(int(bucket.EncryptionParameters.BlockSize)),
		dbx.Bucket_DefaultRedundancyAlgorithm(int(bucket.RedundancyScheme.Algorithm)),
		dbx.Bucket_DefaultRedundancyShareSize(int(bucket.RedundancyScheme.ShareSize)),
		dbx.Bucket_DefaultRedundancyRequiredShares(int(bucket.RedundancyScheme.RequiredShares)),
		dbx.Bucket_DefaultRedundancyRepairShares(int(bucket.RedundancyScheme.RepairShares)),
		dbx.Bucket_DefaultRedundancyOptimalShares(int(bucket.RedundancyScheme.OptimalShares)),
		dbx.Bucket_DefaultRedundancyTotalShares(int(bucket.RedundancyScheme.TotalShares)),
	)
	if err != nil {
		return storj.ErrBucket.Wrap(err)
	}
	return nil
}

// GetBucket returns a bucket from the database
func (db *bucketsDB) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxbucket, err := db.db.Get_Bucket_By_ProjectId_And_Name(ctx,
		dbx.Bucket_ProjectId(projectID[:]),
		dbx.Bucket_Name(bucketName),
	)
	id, err := uuid.Parse(string(dbxbucket.Id))
	if err != nil {
		return bucket, err
	}
	project, err := uuid.Parse(string(dbxbucket.ProjectId))
	if err != nil {
		return bucket, err
	}
	return storj.Bucket{
		ID:                   *id,
		Name:                 string(dbxbucket.Name),
		ProjectID:            *project,
		Created:              dbxbucket.CreatedAt,
		PathCipher:           storj.Cipher(dbxbucket.PathCipher),
		SegmentsSize:         int64(dbxbucket.DefaultSegmentSize),
		RedundancyScheme:     storj.RedundancyScheme{},
		EncryptionParameters: storj.EncryptionParameters{},
	}, err
}

// DeleteBucket deletes a bucket
func (db *bucketsDB) DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_Bucket_By_ProjectId_And_Name(ctx,
		dbx.Bucket_ProjectId(projectID[:]),
		dbx.Bucket_Name(bucketName),
	)
	return err
}

// ListBuckets returns a list of buckets for a project
func (db *bucketsDB) ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions) (buckets []storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	rows, err := db.db.All_Bucket_By_ProjectId(ctx,
		dbx.Bucket_ProjectId(projectID[:]),
	)
	if err != nil {
		return buckets, err
	}
	for _, dbxbucket := range rows {
		id, err := uuid.Parse(string(dbxbucket.Id))
		if err != nil {
			return buckets, err
		}
		project, err := uuid.Parse(string(dbxbucket.ProjectId))
		if err != nil {
			return buckets, err
		}
		item := storj.Bucket{
			ID:                   *id,
			Name:                 string(dbxbucket.Name),
			ProjectID:            *project,
			Created:              dbxbucket.CreatedAt,
			PathCipher:           storj.Cipher(dbxbucket.PathCipher),
			SegmentsSize:         int64(dbxbucket.DefaultSegmentSize),
			RedundancyScheme:     storj.RedundancyScheme{},
			EncryptionParameters: storj.EncryptionParameters{},
		}
		buckets = append(buckets, item)
	}
	return buckets, err
}

// Buckets returns database for interacting with buckets
func (db *DB) Buckets() buckets.DB {
	return &bucketsDB{db: db.db}
}
