// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"errors"

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
		dbx.BucketMetainfo_DefaultSegmentSize(int(bucket.DefaultSegmentsSize)),
		dbx.BucketMetainfo_DefaultEncryptionCipherSuite(int(bucket.DefaultEncryptionParameters.CipherSuite)),
		dbx.BucketMetainfo_DefaultEncryptionBlockSize(int(bucket.DefaultEncryptionParameters.BlockSize)),
		dbx.BucketMetainfo_DefaultRedundancyAlgorithm(int(bucket.DefaultRedundancyScheme.Algorithm)),
		dbx.BucketMetainfo_DefaultRedundancyShareSize(int(bucket.DefaultRedundancyScheme.ShareSize)),
		dbx.BucketMetainfo_DefaultRedundancyRequiredShares(int(bucket.DefaultRedundancyScheme.RequiredShares)),
		dbx.BucketMetainfo_DefaultRedundancyRepairShares(int(bucket.DefaultRedundancyScheme.RepairShares)),
		dbx.BucketMetainfo_DefaultRedundancyOptimalShares(int(bucket.DefaultRedundancyScheme.OptimalShares)),
		dbx.BucketMetainfo_DefaultRedundancyTotalShares(int(bucket.DefaultRedundancyScheme.TotalShares)),
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

// ListBuckets returns a list of buckets for a project
func (db *bucketsDB) ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions) (bucketList storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	const defaultListLimit = 10000
	if listOpts.Limit < 1 {
		listOpts.Limit = defaultListLimit
	}
	limit := listOpts.Limit + 1 // add one to detect More

	var dbxBuckets []*dbx.BucketMetainfo
	switch listOpts.Direction {
	case storj.Before:
		dbxBuckets, err = db.db.Limited_BucketMetainfo_By_ProjectId_And_Name_Less_OrderBy_Desc_Name(ctx,
			dbx.BucketMetainfo_ProjectId(projectID[:]),
			dbx.BucketMetainfo_Name([]byte(listOpts.Cursor)),
			limit,
			0,
		)
	case storj.Backward:
		dbxBuckets, err = db.db.Limited_BucketMetainfo_By_ProjectId_And_Name_LessOrEqual_OrderBy_Desc_Name(ctx,
			dbx.BucketMetainfo_ProjectId(projectID[:]),
			dbx.BucketMetainfo_Name([]byte(listOpts.Cursor)),
			limit,
			0,
		)
	case storj.After:
		dbxBuckets, err = db.db.Limited_BucketMetainfo_By_ProjectId_And_Name_Greater_OrderBy_Asc_Name(ctx,
			dbx.BucketMetainfo_ProjectId(projectID[:]),
			dbx.BucketMetainfo_Name([]byte(listOpts.Cursor)),
			limit,
			0,
		)
	case storj.Forward:
		dbxBuckets, err = db.db.Limited_BucketMetainfo_By_ProjectId_And_Name_GreaterOrEqual_OrderBy_Asc_Name(ctx,
			dbx.BucketMetainfo_ProjectId(projectID[:]),
			dbx.BucketMetainfo_Name([]byte(listOpts.Cursor)),
			limit,
			0,
		)
	default:
		return bucketList, errors.New("unknown list direction")
	}
	if err != nil {
		return bucketList, err
	}

	bucketList.More = len(dbxBuckets) > listOpts.Limit
	if bucketList.More {
		// if there are more buckets than listOpts.limit returned,
		// then remove the extra
		dbxBuckets = dbxBuckets[0:listOpts.Limit]
	}

	bucketList.Items = make([]storj.Bucket, len(dbxBuckets))
	for i, dbxBucket := range dbxBuckets {
		item, err := convertDBXtoBucket(dbxBucket)
		if err != nil {
			return bucketList, err
		}
		bucketList.Items[i] = item
	}

	return bucketList, err
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
		ID:                  id,
		Name:                string(dbxBucket.Name),
		ProjectID:           project,
		Created:             dbxBucket.CreatedAt,
		PathCipher:          storj.CipherSuite(dbxBucket.PathCipher),
		DefaultSegmentsSize: int64(dbxBucket.DefaultSegmentSize),
		DefaultRedundancyScheme: storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(dbxBucket.DefaultRedundancyAlgorithm),
			ShareSize:      int32(dbxBucket.DefaultRedundancyShareSize),
			RequiredShares: int16(dbxBucket.DefaultRedundancyRequiredShares),
			RepairShares:   int16(dbxBucket.DefaultRedundancyRepairShares),
			OptimalShares:  int16(dbxBucket.DefaultRedundancyOptimalShares),
			TotalShares:    int16(dbxBucket.DefaultRedundancyTotalShares),
		},
		DefaultEncryptionParameters: storj.EncryptionParameters{
			CipherSuite: storj.CipherSuite(dbxBucket.DefaultEncryptionCipherSuite),
			BlockSize:   int32(dbxBucket.DefaultEncryptionBlockSize),
		},
	}, err
}
