// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/metainfo"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bucketsDB struct {
	db *satelliteDB
}

// Buckets returns database for interacting with buckets
func (db *satelliteDB) Buckets() metainfo.BucketsDB {
	return &bucketsDB{db: db}
}

// CreateBucket creates a new bucket
func (db *bucketsDB) CreateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	partnerID := dbx.BucketMetainfo_Create_Fields{}
	if !bucket.PartnerID.IsZero() {
		partnerID = dbx.BucketMetainfo_Create_Fields{
			PartnerId: dbx.BucketMetainfo_PartnerId(bucket.PartnerID[:]),
		}
	}

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
		partnerID,
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
func (db *bucketsDB) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxBucket, err := db.db.Get_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return storj.Bucket{}, storj.ErrBucketNotFound.Wrap(err)
		}
		return storj.Bucket{}, storj.ErrBucket.Wrap(err)
	}
	return convertDBXtoBucket(dbxBucket)
}

// UpdateBucket upates a bucket
func (db *bucketsDB) UpdateBucket(ctx context.Context, bucket storj.Bucket) (_ storj.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	if bucket.PartnerID.IsZero() {
		return storj.Bucket{}, Error.New("partnerId is zero")
	}

	var updateFields dbx.BucketMetainfo_Update_Fields
	updateFields.PartnerId = dbx.BucketMetainfo_PartnerId(bucket.PartnerID[:])

	dbxBucket, err := db.db.Update_BucketMetainfo_By_ProjectId_And_Name(ctx, dbx.BucketMetainfo_ProjectId(bucket.ProjectID[:]), dbx.BucketMetainfo_Name([]byte(bucket.Name)), updateFields)
	if err != nil {
		return storj.Bucket{}, storj.ErrBucket.Wrap(err)
	}
	return convertDBXtoBucket(dbxBucket)
}

// DeleteBucket deletes a bucket
func (db *bucketsDB) DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		return storj.ErrBucket.Wrap(err)
	}
	return nil
}

// ListBuckets returns a list of buckets for a project
func (db *bucketsDB) ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts storj.BucketListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList storj.BucketList, err error) {
	defer mon.Task()(&ctx)(&err)

	const defaultListLimit = 10000
	if listOpts.Limit < 1 {
		listOpts.Limit = defaultListLimit
	}
	limit := listOpts.Limit + 1 // add one to detect More

	for {
		var dbxBuckets []*dbx.BucketMetainfo
		switch listOpts.Direction {
		// For simplictiy we are only supporting the forward direction for listing buckets
		case storj.Forward:
			dbxBuckets, err = db.db.Limited_BucketMetainfo_By_ProjectId_And_Name_GreaterOrEqual_OrderBy_Asc_Name(ctx,
				dbx.BucketMetainfo_ProjectId(projectID[:]),
				dbx.BucketMetainfo_Name([]byte(listOpts.Cursor)),
				limit,
				0,
			)

		// After is only called by BucketListOptions.NextPage and is the paginated Forward direction
		case storj.After:
			dbxBuckets, err = db.db.Limited_BucketMetainfo_By_ProjectId_And_Name_Greater_OrderBy_Asc_Name(ctx,
				dbx.BucketMetainfo_ProjectId(projectID[:]),
				dbx.BucketMetainfo_Name([]byte(listOpts.Cursor)),
				limit,
				0,
			)
		default:
			return bucketList, errors.New("unknown list direction")
		}
		if err != nil {
			return bucketList, storj.ErrBucket.Wrap(err)
		}

		bucketList.More = len(dbxBuckets) > listOpts.Limit
		if bucketList.More {
			// If there are more buckets than listOpts.limit returned,
			// then remove the extra buckets so that we do not return
			// more then the limit
			dbxBuckets = dbxBuckets[0:listOpts.Limit]
		}

		if bucketList.Items == nil {
			bucketList.Items = make([]storj.Bucket, 0, len(dbxBuckets))
		}

		for _, dbxBucket := range dbxBuckets {
			// Check that the bucket is allowed to be viewed
			_, bucketAllowed := allowedBuckets.Buckets[string(dbxBucket.Name)]
			if bucketAllowed || allowedBuckets.All {
				item, err := convertDBXtoBucket(dbxBucket)
				if err != nil {
					return bucketList, storj.ErrBucket.Wrap(err)
				}
				bucketList.Items = append(bucketList.Items, item)
			}
		}

		if len(bucketList.Items) < listOpts.Limit && bucketList.More {
			// If we filtered out disallowed buckets, then get more buckets
			// out of database so that we return `limit` number of buckets
			listOpts = storj.BucketListOptions{
				Cursor:    string(dbxBuckets[len(dbxBuckets)-1].Name),
				Limit:     listOpts.Limit,
				Direction: storj.After,
			}
			continue
		}
		break
	}

	return bucketList, nil
}

func convertDBXtoBucket(dbxBucket *dbx.BucketMetainfo) (bucket storj.Bucket, err error) {
	id, err := dbutil.BytesToUUID(dbxBucket.Id)
	if err != nil {
		return bucket, storj.ErrBucket.Wrap(err)
	}
	project, err := dbutil.BytesToUUID(dbxBucket.ProjectId)
	if err != nil {
		return bucket, storj.ErrBucket.Wrap(err)
	}

	bucket = storj.Bucket{
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
	}

	if dbxBucket.PartnerId != nil {
		partnerID, err := dbutil.BytesToUUID(dbxBucket.PartnerId)
		if err != nil {
			return bucket, storj.ErrBucket.Wrap(err)
		}
		bucket.PartnerID = partnerID
	}

	return bucket, nil
}
