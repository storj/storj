// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type bucketsDB struct {
	db *satelliteDB
}

// CreateBucket creates a new bucket.
func (db *bucketsDB) CreateBucket(ctx context.Context, bucket buckets.Bucket) (_ buckets.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	optionalFields := dbx.BucketMetainfo_Create_Fields{}
	if bucket.UserAgent != nil {
		optionalFields = dbx.BucketMetainfo_Create_Fields{
			UserAgent: dbx.BucketMetainfo_UserAgent(bucket.UserAgent),
		}
	}
	if bucket.Versioning != buckets.VersioningUnsupported {
		optionalFields = dbx.BucketMetainfo_Create_Fields{
			Versioning: dbx.BucketMetainfo_Versioning(int(bucket.Versioning)),
		}
	}
	optionalFields.Placement = dbx.BucketMetainfo_Placement(int(bucket.Placement))

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
		optionalFields,
	)
	if err != nil {
		if dbx.IsConstraintError(err) {
			return buckets.Bucket{}, buckets.ErrBucketAlreadyExists.New("")
		}
		return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
	}

	bucket, err = convertDBXtoBucket(row)
	if err != nil {
		return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
	}
	return bucket, nil
}

// GetBucket returns a bucket.
func (db *bucketsDB) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ buckets.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxBucket, err := db.db.Get_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return buckets.Bucket{}, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
	}
	return convertDBXtoBucket(dbxBucket)
}

// GetBucketPlacement returns with the placement constraint identifier.
func (db *bucketsDB) GetBucketPlacement(ctx context.Context, bucketName []byte, projectID uuid.UUID) (placement storj.PlacementConstraint, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxPlacement, err := db.db.Get_BucketMetainfo_Placement_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storj.EveryCountry, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return storj.EveryCountry, buckets.ErrBucket.Wrap(err)
	}
	placement = storj.EveryCountry
	if dbxPlacement.Placement != nil {
		placement = storj.PlacementConstraint(*dbxPlacement.Placement)
	}

	return placement, nil
}

// GetBucketVersioningState returns with the versioning state of the bucket.
func (db *bucketsDB) GetBucketVersioningState(ctx context.Context, bucketName []byte, projectID uuid.UUID) (versioningState buckets.Versioning, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxVersioning, err := db.db.Get_BucketMetainfo_Versioning_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return -1, buckets.ErrBucket.Wrap(err)
	}

	return buckets.Versioning(dbxVersioning.Versioning), nil
}

// EnableBucketVersioning enables versioning for a bucket.
func (db *bucketsDB) EnableBucketVersioning(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	dbxBucket, err := db.db.Update_BucketMetainfo_By_ProjectId_And_Name_And_Versioning_GreaterOrEqual(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
		// only enable versioning if current versioning state is unversioned, enabled, or suspended.
		dbx.BucketMetainfo_Versioning(int(buckets.Unversioned)),
		dbx.BucketMetainfo_Update_Fields{
			Versioning: dbx.BucketMetainfo_Versioning(int(buckets.VersioningEnabled)),
		})
	if err != nil {
		return buckets.ErrBucket.Wrap(err)
	}
	if dbxBucket == nil {
		return buckets.ErrBucketNotFound.New("%s", bucketName)
	}
	if buckets.Versioning(dbxBucket.Versioning) != buckets.VersioningEnabled {
		return buckets.ErrBucket.New("cannot transition bucket versioning state to enabled")
	}
	return nil
}

// SuspendBucketVersioning disables versioning for a bucket.
func (db *bucketsDB) SuspendBucketVersioning(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	dbxBucket, err := db.db.Update_BucketMetainfo_By_ProjectId_And_Name_And_Versioning_GreaterOrEqual(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
		// only suspend versioning if current versioning state is enabled, or suspended.
		dbx.BucketMetainfo_Versioning(int(buckets.VersioningEnabled)),
		dbx.BucketMetainfo_Update_Fields{
			Versioning: dbx.BucketMetainfo_Versioning(int(buckets.VersioningSuspended)),
		})
	if err != nil {
		return buckets.ErrBucket.Wrap(err)
	}
	if dbxBucket == nil {
		return buckets.ErrBucketNotFound.New("%s", bucketName)
	}
	if buckets.Versioning(dbxBucket.Versioning) != buckets.VersioningSuspended {
		return buckets.ErrBucket.New("cannot transition bucket versioning state to suspended")
	}
	return nil
}

// GetMinimalBucket returns existing bucket with minimal number of fields.
func (db *bucketsDB) GetMinimalBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ buckets.MinimalBucket, err error) {
	defer mon.Task()(&ctx)(&err)
	row, err := db.db.Get_BucketMetainfo_CreatedAt_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return buckets.MinimalBucket{}, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return buckets.MinimalBucket{}, buckets.ErrBucket.Wrap(err)
	}
	return buckets.MinimalBucket{
		Name:      bucketName,
		CreatedAt: row.CreatedAt,
	}, nil
}

// HasBucket returns if a bucket exists.
func (db *bucketsDB) HasBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (exists bool, err error) {
	defer mon.Task()(&ctx)(&err)

	exists, err = db.db.Has_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	return exists, buckets.ErrBucket.Wrap(err)
}

// UpdateBucket updates a bucket.
func (db *bucketsDB) UpdateBucket(ctx context.Context, bucket buckets.Bucket) (_ buckets.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	var updateFields dbx.BucketMetainfo_Update_Fields

	if bucket.UserAgent != nil {
		updateFields.UserAgent = dbx.BucketMetainfo_UserAgent(bucket.UserAgent)
	}

	updateFields.Placement = dbx.BucketMetainfo_Placement(int(bucket.Placement))

	dbxBucket, err := db.db.Update_BucketMetainfo_By_ProjectId_And_Name(ctx, dbx.BucketMetainfo_ProjectId(bucket.ProjectID[:]), dbx.BucketMetainfo_Name([]byte(bucket.Name)), updateFields)
	if err != nil {
		return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
	}
	return convertDBXtoBucket(dbxBucket)
}

// UpdateUserAgent updates buckets user agent.
func (db *bucketsDB) UpdateUserAgent(ctx context.Context, projectID uuid.UUID, bucketName string, userAgent []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Update_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name([]byte(bucketName)),
		dbx.BucketMetainfo_Update_Fields{
			UserAgent: dbx.BucketMetainfo_UserAgent(userAgent),
		})

	return err
}

// DeleteBucket deletes a bucket.
func (db *bucketsDB) DeleteBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	deleted, err := db.db.Delete_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		return buckets.ErrBucket.Wrap(err)
	}
	if !deleted {
		return buckets.ErrBucketNotFound.New("%s", bucketName)
	}
	return nil
}

// ListBuckets returns a list of buckets for a project.
func (db *bucketsDB) ListBuckets(ctx context.Context, projectID uuid.UUID, listOpts buckets.ListOptions, allowedBuckets macaroon.AllowedBuckets) (bucketList buckets.List, err error) {
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
		case buckets.DirectionForward:
			dbxBuckets, err = db.db.Limited_BucketMetainfo_By_ProjectId_And_Name_GreaterOrEqual_OrderBy_Asc_Name(ctx,
				dbx.BucketMetainfo_ProjectId(projectID[:]),
				dbx.BucketMetainfo_Name([]byte(listOpts.Cursor)),
				limit,
				0,
			)

		// After is only called by BucketListOptions.NextPage and is the paginated Forward direction
		case buckets.DirectionAfter:
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
			return bucketList, buckets.ErrBucket.Wrap(err)
		}

		bucketList.More = len(dbxBuckets) > listOpts.Limit
		if bucketList.More {
			// If there are more buckets than listOpts.limit returned,
			// then remove the extra buckets so that we do not return
			// more then the limit
			dbxBuckets = dbxBuckets[0:listOpts.Limit]
		}

		if bucketList.Items == nil {
			bucketList.Items = make([]buckets.Bucket, 0, len(dbxBuckets))
		}

		for _, dbxBucket := range dbxBuckets {
			// Check that the bucket is allowed to be viewed
			_, bucketAllowed := allowedBuckets.Buckets[string(dbxBucket.Name)]
			if bucketAllowed || allowedBuckets.All {
				item, err := convertDBXtoBucket(dbxBucket)
				if err != nil {
					return bucketList, buckets.ErrBucket.Wrap(err)
				}
				bucketList.Items = append(bucketList.Items, item)
			}
		}

		if len(bucketList.Items) < listOpts.Limit && bucketList.More {
			// If we filtered out disallowed buckets, then get more buckets
			// out of database so that we return `limit` number of buckets
			listOpts = buckets.ListOptions{
				Cursor:    string(dbxBuckets[len(dbxBuckets)-1].Name),
				Limit:     listOpts.Limit,
				Direction: buckets.DirectionAfter,
			}
			continue
		}
		break
	}

	return bucketList, nil
}

// CountBuckets returns the number of buckets a project currently has.
func (db *bucketsDB) CountBuckets(ctx context.Context, projectID uuid.UUID) (count int, err error) {
	count64, err := db.db.Count_BucketMetainfo_Name_By_ProjectId(ctx, dbx.BucketMetainfo_ProjectId(projectID[:]))
	if err != nil {
		return -1, err
	}
	return int(count64), nil
}

func convertDBXtoBucket(dbxBucket *dbx.BucketMetainfo) (bucket buckets.Bucket, err error) {
	id, err := uuid.FromBytes(dbxBucket.Id)
	if err != nil {
		return bucket, buckets.ErrBucket.Wrap(err)
	}
	project, err := uuid.FromBytes(dbxBucket.ProjectId)
	if err != nil {
		return bucket, buckets.ErrBucket.Wrap(err)
	}

	bucket = buckets.Bucket{
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
		Versioning: buckets.Versioning(dbxBucket.Versioning),
	}

	if dbxBucket.Placement != nil {
		bucket.Placement = storj.PlacementConstraint(*dbxBucket.Placement)
	}

	if dbxBucket.UserAgent != nil {
		bucket.UserAgent = dbxBucket.UserAgent
	}

	return bucket, nil
}

// IterateBucketLocations iterates through all buckets with specific page size.
func (db *bucketsDB) IterateBucketLocations(ctx context.Context, pageSize int, fn func([]metabase.BucketLocation) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	page := make([]metabase.BucketLocation, pageSize)

	var continuationToken *dbx.Paged_BucketMetainfo_ProjectId_BucketMetainfo_Name_Continuation
	var rows []*dbx.ProjectId_Name_Row
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		rows, continuationToken, err = db.db.Paged_BucketMetainfo_ProjectId_BucketMetainfo_Name(ctx, pageSize, continuationToken)
		if err != nil {
			return Error.Wrap(err)
		}

		if len(rows) == 0 {
			return nil
		}

		for i, row := range rows {
			projectID, err := uuid.FromBytes(row.ProjectId)
			if err != nil {
				return Error.Wrap(err)
			}

			page[i].ProjectID = projectID
			page[i].BucketName = string(row.Name)
		}

		if err := fn(page[:len(rows)]); err != nil {
			return Error.Wrap(err)
		}

	}
}
