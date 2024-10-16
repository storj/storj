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

	optionalFields := dbx.BucketMetainfo_Create_Fields{
		Placement:         dbx.BucketMetainfo_Placement(int(bucket.Placement)),
		ObjectLockEnabled: dbx.BucketMetainfo_ObjectLockEnabled(bucket.ObjectLock.Enabled),
	}
	if bucket.UserAgent != nil {
		optionalFields.UserAgent = dbx.BucketMetainfo_UserAgent(bucket.UserAgent)
	}
	if bucket.Versioning != buckets.VersioningUnsupported {
		optionalFields.Versioning = dbx.BucketMetainfo_Versioning(int(bucket.Versioning))
	}
	if !bucket.CreatedBy.IsZero() {
		optionalFields.CreatedBy = dbx.BucketMetainfo_CreatedBy(bucket.CreatedBy[:])
	}

	if bucket.ObjectLock.DefaultRetentionMode != storj.NoRetention {
		if !bucket.ObjectLock.Enabled {
			return buckets.Bucket{}, buckets.ErrBucket.New("default retention mode must not be set if Object Lock is not enabled")
		}
		if bucket.ObjectLock.DefaultRetentionDays == 0 && bucket.ObjectLock.DefaultRetentionYears == 0 {
			return buckets.Bucket{}, buckets.ErrBucket.New("default retention mode must not be set without a default retention duration")
		}
		if bucket.ObjectLock.DefaultRetentionDays != 0 && bucket.ObjectLock.DefaultRetentionYears != 0 {
			return buckets.Bucket{}, buckets.ErrBucket.New("default retention days and years must not be set simultaneously")
		}

		optionalFields.DefaultRetentionMode = dbx.BucketMetainfo_DefaultRetentionMode(int(bucket.ObjectLock.DefaultRetentionMode))

		if bucket.ObjectLock.DefaultRetentionDays != 0 {
			if bucket.ObjectLock.DefaultRetentionDays < 0 {
				return buckets.Bucket{}, buckets.ErrBucket.New("default retention days must be positive")
			}
			optionalFields.DefaultRetentionDays = dbx.BucketMetainfo_DefaultRetentionDays(bucket.ObjectLock.DefaultRetentionDays)
		}

		if bucket.ObjectLock.DefaultRetentionYears != 0 {
			if bucket.ObjectLock.DefaultRetentionYears < 0 {
				return buckets.Bucket{}, buckets.ErrBucket.New("default retention years must be positive")
			}
			optionalFields.DefaultRetentionYears = dbx.BucketMetainfo_DefaultRetentionYears(bucket.ObjectLock.DefaultRetentionYears)
		}
	} else if bucket.ObjectLock.DefaultRetentionDays != 0 || bucket.ObjectLock.DefaultRetentionYears != 0 {
		return buckets.Bucket{}, buckets.ErrBucket.New("default retention duration must not be set without a default retention mode")
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

	dbxBucket, err := db.db.Get_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return buckets.ErrBucket.Wrap(err)
	}
	if dbxBucket.Versioning == int(buckets.VersioningUnsupported) {
		return buckets.ErrConflict.New("versioning is unsupported for this bucket")
	}

	_, err = db.db.Update_BucketMetainfo_By_ProjectId_And_Name_And_Versioning_GreaterOrEqual(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
		// Only enable versioning if current versioning state is unversioned, enabled, or suspended.
		dbx.BucketMetainfo_Versioning(int(buckets.Unversioned)),
		dbx.BucketMetainfo_Update_Fields{
			Versioning: dbx.BucketMetainfo_Versioning(int(buckets.VersioningEnabled)),
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return buckets.ErrBucket.Wrap(err)
	}

	return nil
}

// SuspendBucketVersioning disables versioning for a bucket.
func (db *bucketsDB) SuspendBucketVersioning(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	dbxBucket, err := db.db.Get_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return buckets.ErrBucket.Wrap(err)
	}
	if dbxBucket.Versioning < int(buckets.VersioningEnabled) {
		return buckets.ErrConflict.New("versioning may only be suspended for buckets with versioning enabled")
	}
	if dbxBucket.ObjectLockEnabled {
		return buckets.ErrLocked.New("versioning may not be suspended for buckets with Object Lock enabled")
	}

	_, err = db.db.Update_BucketMetainfo_By_ProjectId_And_Name_And_Versioning_GreaterOrEqual_And_ObjectLockEnabled_Equal_False(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
		// Only suspend versioning if current versioning state is enabled or suspended.
		dbx.BucketMetainfo_Versioning(int(buckets.VersioningEnabled)),
		dbx.BucketMetainfo_Update_Fields{
			Versioning: dbx.BucketMetainfo_Versioning(int(buckets.VersioningSuspended)),
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// This should only occur if, between the execution of the two queries,
			// the bucket was deleted or Object Lock was enabled for it.
			return buckets.ErrUnavailable.New("")
		}
		return buckets.ErrBucket.Wrap(err)
	}

	return nil
}

// GetMinimalBucket returns existing bucket with minimal number of fields.
func (db *bucketsDB) GetMinimalBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ buckets.MinimalBucket, err error) {
	defer mon.Task()(&ctx)(&err)
	row, err := db.db.Get_BucketMetainfo_CreatedBy_BucketMetainfo_CreatedAt_BucketMetainfo_Placement_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return buckets.MinimalBucket{}, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return buckets.MinimalBucket{}, buckets.ErrBucket.Wrap(err)
	}

	var createdBy uuid.UUID
	if row.CreatedBy != nil {
		createdBy, err = uuid.FromBytes(row.CreatedBy)
		if err != nil {
			return buckets.MinimalBucket{}, buckets.ErrBucket.Wrap(err)
		}
	}

	var placement storj.PlacementConstraint
	if row.Placement != nil {
		placement = storj.PlacementConstraint(*row.Placement)
	}

	return buckets.MinimalBucket{
		Name:      bucketName,
		CreatedBy: createdBy,
		CreatedAt: row.CreatedAt,
		Placement: placement,
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

// UpdateBucketObjectLockSettings updates object lock settings for a bucket without an extra database query.
func (db *bucketsDB) UpdateBucketObjectLockSettings(ctx context.Context, params buckets.UpdateBucketObjectLockParams) (_ buckets.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	var updateFields dbx.BucketMetainfo_Update_Fields

	if params.ObjectLockEnabled {
		updateFields.ObjectLockEnabled = dbx.BucketMetainfo_ObjectLockEnabled(true)
	} else {
		return buckets.Bucket{}, buckets.ErrBucket.New("object lock cannot be disabled")
	}
	if params.DefaultRetentionMode != nil {
		if *params.DefaultRetentionMode == nil || **params.DefaultRetentionMode == storj.NoRetention {
			updateFields.DefaultRetentionMode = dbx.BucketMetainfo_DefaultRetentionMode_Null()
		} else {
			updateFields.DefaultRetentionMode = dbx.BucketMetainfo_DefaultRetentionMode(int(**params.DefaultRetentionMode))
		}
	}

	var (
		daysSetToPositiveValue  bool
		yearsSetToPositiveValue bool
	)

	if params.DefaultRetentionDays != nil {
		if *params.DefaultRetentionDays == nil || **params.DefaultRetentionDays == 0 {
			updateFields.DefaultRetentionDays = dbx.BucketMetainfo_DefaultRetentionDays_Null()
		} else {
			days := **params.DefaultRetentionDays
			if days < 0 {
				return buckets.Bucket{}, buckets.ErrBucket.New("default retention days must be a positive integer")
			}

			updateFields.DefaultRetentionDays = dbx.BucketMetainfo_DefaultRetentionDays(days)
			updateFields.DefaultRetentionYears = dbx.BucketMetainfo_DefaultRetentionYears_Null()
			daysSetToPositiveValue = true
		}
	}

	if params.DefaultRetentionYears != nil {
		if *params.DefaultRetentionYears == nil || **params.DefaultRetentionYears == 0 {
			updateFields.DefaultRetentionYears = dbx.BucketMetainfo_DefaultRetentionYears_Null()
		} else {
			years := **params.DefaultRetentionYears
			if years < 0 {
				return buckets.Bucket{}, buckets.ErrBucket.New("default retention years must be a positive integer")
			}

			updateFields.DefaultRetentionYears = dbx.BucketMetainfo_DefaultRetentionYears(years)
			updateFields.DefaultRetentionDays = dbx.BucketMetainfo_DefaultRetentionDays_Null()
			yearsSetToPositiveValue = true
		}
	}

	if daysSetToPositiveValue && yearsSetToPositiveValue {
		return buckets.Bucket{}, buckets.ErrBucket.New("only one of default_retention_days or default_retention_years can be set to a positive value")
	}

	dbxBucket, err := db.db.Update_BucketMetainfo_By_ProjectId_And_Name(
		ctx,
		dbx.BucketMetainfo_ProjectId(params.ProjectID[:]),
		dbx.BucketMetainfo_Name([]byte(params.Name)),
		updateFields,
	)
	if err != nil {
		return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
	}
	if dbxBucket == nil {
		return buckets.Bucket{}, buckets.ErrBucketNotFound.New("%s", params.Name)
	}

	return convertDBXtoBucket(dbxBucket)
}

// GetBucketObjectLockSettings returns a bucket's object lock settings.
func (db *bucketsDB) GetBucketObjectLockSettings(ctx context.Context, bucketName []byte, projectID uuid.UUID) (settings *buckets.ObjectLockSettings, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxSettings, err := db.db.Get_BucketMetainfo_ObjectLockEnabled_BucketMetainfo_DefaultRetentionMode_BucketMetainfo_DefaultRetentionDays_BucketMetainfo_DefaultRetentionYears_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return nil, buckets.ErrBucket.Wrap(err)
	}

	settings = &buckets.ObjectLockSettings{
		Enabled: dbxSettings.ObjectLockEnabled,
	}

	if dbxSettings.DefaultRetentionMode != nil {
		settings.DefaultRetentionMode = storj.RetentionMode(*dbxSettings.DefaultRetentionMode)
	}
	if dbxSettings.DefaultRetentionDays != nil {
		settings.DefaultRetentionDays = *dbxSettings.DefaultRetentionDays
	}
	if dbxSettings.DefaultRetentionYears != nil {
		settings.DefaultRetentionYears = *dbxSettings.DefaultRetentionYears
	}

	return settings, nil
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
			return bucketList, Error.New("unknown list direction")
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

	var createdBy uuid.UUID
	if dbxBucket.CreatedBy != nil {
		createdBy, err = uuid.FromBytes(dbxBucket.CreatedBy)
		if err != nil {
			return bucket, buckets.ErrBucket.Wrap(err)
		}
	}

	bucket = buckets.Bucket{
		ID:                  id,
		Name:                string(dbxBucket.Name),
		ProjectID:           project,
		Created:             dbxBucket.CreatedAt,
		CreatedBy:           createdBy,
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
		ObjectLock: buckets.ObjectLockSettings{
			Enabled: dbxBucket.ObjectLockEnabled,
		},
	}

	if dbxBucket.Placement != nil {
		bucket.Placement = storj.PlacementConstraint(*dbxBucket.Placement)
	}

	if dbxBucket.UserAgent != nil {
		bucket.UserAgent = dbxBucket.UserAgent
	}

	if dbxBucket.DefaultRetentionMode != nil {
		bucket.ObjectLock.DefaultRetentionMode = storj.RetentionMode(*dbxBucket.DefaultRetentionMode)
	}
	if dbxBucket.DefaultRetentionDays != nil {
		bucket.ObjectLock.DefaultRetentionDays = *dbxBucket.DefaultRetentionDays
	}
	if dbxBucket.DefaultRetentionYears != nil {
		bucket.ObjectLock.DefaultRetentionYears = *dbxBucket.DefaultRetentionYears
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
			page[i].BucketName = metabase.BucketName(row.Name)
		}

		if err := fn(page[:len(rows)]); err != nil {
			return Error.Wrap(err)
		}

	}
}

// GetBucketObjectLockEnabled returns whether a bucket has Object Lock enabled.
func (db *bucketsDB) GetBucketObjectLockEnabled(ctx context.Context, bucketName []byte, projectID uuid.UUID) (enabled bool, err error) {
	defer mon.Task()(&ctx)(&err)
	row, err := db.db.Get_BucketMetainfo_ObjectLockEnabled_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return false, buckets.ErrBucket.Wrap(err)
	}
	return row.ObjectLockEnabled, nil
}
