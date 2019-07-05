// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bucketsDB struct {
	db dbx.Methods
}

// Buckets returns database for interacting with buckets
func (db *DB) Buckets() metainfo.BucketsDB {
	return &bucketsDB{db: db.db}
}

// CreateBucket creates a new bucket
func (db *bucketsDB) CreateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	row, err := db.db.Create_BucketMetainfo(ctx,
		dbx.BucketMetainfo_Id(bucket.ID[:]),
		dbx.BucketMetainfo_ProjectId(bucket.ProjectID[:]),
		dbx.BucketMetainfo_Name([]byte(bucket.Name)),
		dbx.BucketMetainfo_PathCipher(int(bucket.PathCipher)),
		dbx.BucketMetainfo_DefaultSegmentSize(int(bucket.SegmentsSize)),
		dbx.BucketMetainfo_DefaultEncryptionCipherSuite(int(bucket.EncryptionParameters.CipherSuite)),
		dbx.BucketMetainfo_DefaultEncryptionBlockSize(int(bucket.EncryptionParameters.BlockSize)),
		dbx.BucketMetainfo_DefaultRedundancyAlgorithm(int(bucket.RedundancyScheme.Algorithm)),
		dbx.BucketMetainfo_DefaultRedundancyShareSize(int(bucket.RedundancyScheme.ShareSize)),
		dbx.BucketMetainfo_DefaultRedundancyRequiredShares(int(bucket.RedundancyScheme.RequiredShares)),
		dbx.BucketMetainfo_DefaultRedundancyRepairShares(int(bucket.RedundancyScheme.RepairShares)),
		dbx.BucketMetainfo_DefaultRedundancyOptimalShares(int(bucket.RedundancyScheme.OptimalShares)),
		dbx.BucketMetainfo_DefaultRedundancyTotalShares(int(bucket.RedundancyScheme.TotalShares)),
	)
	if err != nil {
		return storj.Bucket{}, storj.ErrBucket.Wrap(err)
	}

	bucket, err = convertDBXtoBucket(row)
	if err != nil {
		return storj.Bucket{}, storj.ErrBucket.Wrap(err)
	}
	return bucket, nil
}

// GetBucket returns a bucket
func (db *bucketsDB) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxBucket, err := db.db.Get_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	return convertDBXtoBucket(dbxBucket)
}

// DeleteBucket deletes a bucket
func (db *bucketsDB) DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	return err
}

// ListBuckets returns a list of all buckets for a project
func (db *bucketsDB) ListBuckets(ctx context.Context, projectID uuid.UUID) (buckets []storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	rows, err := db.db.All_BucketMetainfo_By_ProjectId_OrderBy_Asc_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
	)
	if err != nil {
		return buckets, err
	}
	for _, dbxBucket := range rows {
		item, err := convertDBXtoBucket(dbxBucket)
		if err != nil {
			return buckets, err
		}
		buckets = append(buckets, item)
	}
	return buckets, err
}

func convertDBXtoBucket(dbxBucket *dbx.BucketMetainfo) (bucket storj.Bucket, err error) {
	id, err := bytesToUUID(dbxBucket.Id)
	if err != nil {
		return bucket, err
	}
	project, err := bytesToUUID(dbxBucket.ProjectId)
	if err != nil {
		return bucket, err
	}
	return storj.Bucket{
		ID:           id,
		Name:         string(dbxBucket.Name),
		ProjectID:    project,
		Created:      dbxBucket.CreatedAt,
		PathCipher:   storj.CipherSuite(dbxBucket.PathCipher),
		SegmentsSize: int64(dbxBucket.DefaultSegmentSize),
		RedundancyScheme: storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(dbxBucket.DefaultRedundancyAlgorithm),
			ShareSize:      int32(dbxBucket.DefaultRedundancyShareSize),
			RequiredShares: int16(dbxBucket.DefaultRedundancyRequiredShares),
			RepairShares:   int16(dbxBucket.DefaultRedundancyRepairShares),
			OptimalShares:  int16(dbxBucket.DefaultRedundancyOptimalShares),
			TotalShares:    int16(dbxBucket.DefaultRedundancyTotalShares),
		},
		EncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(dbxBucket.DefaultEncryptionCipherSuite),
			BlockSize:   int32(dbxBucket.DefaultEncryptionBlockSize),
		},
	}, err
}
