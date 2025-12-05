// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"cloud.google.com/go/spanner"
	"github.com/jackc/pgtype"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
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
		dbx.BucketMetainfo_PathCipher(0),
		dbx.BucketMetainfo_DefaultSegmentSize(0),
		dbx.BucketMetainfo_DefaultEncryptionCipherSuite(0),
		dbx.BucketMetainfo_DefaultEncryptionBlockSize(0),
		dbx.BucketMetainfo_DefaultRedundancyAlgorithm(0),
		dbx.BucketMetainfo_DefaultRedundancyShareSize(0),
		dbx.BucketMetainfo_DefaultRedundancyRequiredShares(0),
		dbx.BucketMetainfo_DefaultRedundancyRepairShares(0),
		dbx.BucketMetainfo_DefaultRedundancyOptimalShares(0),
		dbx.BucketMetainfo_DefaultRedundancyTotalShares(0),
		optionalFields,
	)
	if err != nil {
		if dbx.IsConstraintError(err) {
			return buckets.Bucket{}, buckets.ErrBucketAlreadyExists.New("")
		}
		return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
	}

	bucket, err = convertFullDBXtoBucket(row)
	if err != nil {
		return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
	}
	return bucket, nil
}

// GetBucket returns a bucket.
func (db *bucketsDB) GetBucket(ctx context.Context, bucketName []byte, projectID uuid.UUID) (bucket buckets.Bucket, err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Cockroach, dbutil.Postgres:
		dbxBucket, err := db.db.Get_Bucket(ctx,
			dbx.BucketMetainfo_ProjectId(projectID[:]),
			dbx.BucketMetainfo_Name(bucketName),
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return buckets.Bucket{}, buckets.ErrBucketNotFound.New("%s", bucketName)
			}
			return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
		}
		return convertDBXtoBucket(projectID, string(bucketName), dbxBucket)
	case dbutil.Spanner:
		bucket.ProjectID = projectID
		bucket.Name = string(bucketName)
		var createdBy []byte
		err = spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) (err error) {
			row, err := client.Single().ReadRow(ctx, "bucket_metainfos", spanner.Key{projectID[:], bucketName}, []string{
				"id", "created_by", "user_agent", "created_at", "placement", "versioning",
				"object_lock_enabled", "default_retention_mode", "default_retention_days",
				"default_retention_years",
			})
			if err != nil {
				return err
			}

			return row.Columns(&bucket.ID, &createdBy, &bucket.UserAgent, &bucket.Created, &bucket.Placement, spannerutil.Int(&bucket.Versioning),
				&bucket.ObjectLock.Enabled, spannerutil.Int(&bucket.ObjectLock.DefaultRetentionMode), spannerutil.Int(&bucket.ObjectLock.DefaultRetentionDays),
				spannerutil.Int(&bucket.ObjectLock.DefaultRetentionYears))
		})
		if err != nil {
			if errors.Is(err, spanner.ErrRowNotFound) {
				return buckets.Bucket{}, buckets.ErrBucketNotFound.New("%s", bucketName)
			}
			return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
		}
		// TODO add uuid.UUID support for nullable bytes
		if createdBy != nil {
			bucket.CreatedBy, err = uuid.FromBytes(createdBy)
			if err != nil {
				return buckets.Bucket{}, buckets.ErrBucket.Wrap(err)
			}
		}

		return bucket, nil
	default:
		return buckets.Bucket{}, Error.New("unsupported implementation")
	}
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
	return convertFullDBXtoBucket(dbxBucket)
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

	return convertFullDBXtoBucket(dbxBucket)
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
		// For simplicity we are only supporting the forward direction for listing buckets
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
				item, err := convertFullDBXtoBucket(dbxBucket)
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

// CountObjectLockBuckets returns the number of buckets a project currently has with object lock enabled.
func (db *bucketsDB) CountObjectLockBuckets(ctx context.Context, projectID uuid.UUID) (count int, err error) {
	count64, err := db.db.Count_BucketMetainfo_Name_By_ProjectId_And_ObjectLockEnabled_Equal_True(ctx, dbx.BucketMetainfo_ProjectId(projectID[:]))
	if err != nil {
		return -1, err
	}
	return int(count64), nil
}

func convertFullDBXtoBucket(dbxBucket *dbx.BucketMetainfo) (bucket buckets.Bucket, err error) {
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
		ID:         id,
		Name:       string(dbxBucket.Name),
		ProjectID:  project,
		Created:    dbxBucket.CreatedAt,
		CreatedBy:  createdBy,
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

func convertDBXtoBucket(projectID uuid.UUID, name string, dbxBucket *dbx.Id_CreatedBy_UserAgent_CreatedAt_Placement_Versioning_ObjectLockEnabled_DefaultRetentionMode_DefaultRetentionDays_DefaultRetentionYears_Row) (bucket buckets.Bucket, err error) {
	id, err := uuid.FromBytes(dbxBucket.Id)
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
		ID:         id,
		Name:       name,
		ProjectID:  projectID,
		Created:    dbxBucket.CreatedAt,
		CreatedBy:  createdBy,
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

// GetBucketTagging returns the set of tags placed on a bucket.
func (db *bucketsDB) GetBucketTagging(ctx context.Context, bucketName []byte, projectID uuid.UUID) (tags []buckets.Tag, err error) {
	defer mon.Task()(&ctx)(&err)
	row, err := db.db.Get_BucketMetainfo_Tags_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, buckets.ErrBucketNotFound.New("%s", bucketName)
		}
		return nil, buckets.ErrBucket.Wrap(err)
	}

	tags, err = decodeBucketTags(row.Tags)
	if err != nil {
		return nil, buckets.ErrBucket.New("error decoding tags: %w", err)
	}
	return tags, nil
}

// SetBucketTagging places a set of tags on a bucket.
func (db *bucketsDB) SetBucketTagging(ctx context.Context, bucketName []byte, projectID uuid.UUID, tags []buckets.Tag) (err error) {
	defer mon.Task()(&ctx)(&err)

	var updateFields dbx.BucketMetainfo_Update_Fields
	if len(tags) == 0 {
		updateFields.Tags = dbx.BucketMetainfo_Tags_Null()
	} else {
		encodedTags, err := encodeBucketTags(tags)
		if err != nil {
			return buckets.ErrBucket.New("error encoding tags: %w", err)
		}
		updateFields.Tags = dbx.BucketMetainfo_Tags(encodedTags)
	}

	dbxBucket, err := db.db.Update_BucketMetainfo_By_ProjectId_And_Name(ctx,
		dbx.BucketMetainfo_ProjectId(projectID[:]),
		dbx.BucketMetainfo_Name(bucketName),
		updateFields,
	)
	if err != nil {
		return buckets.ErrBucket.Wrap(err)
	}
	if dbxBucket == nil {
		return buckets.ErrBucketNotFound.New("%s", bucketName)
	}
	return nil
}

// GetBucketNotificationConfig retrieves the notification configuration for a bucket.
// Returns nil if no configuration exists.
func (db *bucketsDB) GetBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (_ *buckets.NotificationConfig, err error) {
	defer mon.Task()(&ctx)(&err)

	var config buckets.NotificationConfig

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		// PostgreSQL handles text arrays via pgtype.TextArray
		var pgEvents pgtype.TextArray
		err = db.db.QueryRowContext(ctx, `
			SELECT config_id, topic_name, events, filter_prefix, filter_suffix, created_at, updated_at
			FROM bucket_eventing_configs
			WHERE project_id = $1 AND bucket_name = $2
		`, projectID[:], bucketName).Scan(
			&config.ConfigID,
			&config.TopicName,
			&pgEvents,
			&config.FilterPrefix,
			&config.FilterSuffix,
			&config.CreatedAt,
			&config.UpdatedAt,
		)

		// Convert pgtype.TextArray to []string
		if err == nil {
			config.Events = make([]string, len(pgEvents.Elements))
			for i, elem := range pgEvents.Elements {
				config.Events[i] = elem.String
			}
		}

	case dbutil.Spanner:
		// Spanner handles ARRAY<STRING> natively but returns []spanner.NullString
		var spannerEvents []spanner.NullString
		err = db.db.QueryRowContext(ctx, `
			SELECT config_id, topic_name, events, filter_prefix, filter_suffix, created_at, updated_at
			FROM bucket_eventing_configs
			WHERE project_id = @project_id AND bucket_name = @bucket_name
		`, sql.Named("project_id", projectID.Bytes()), sql.Named("bucket_name", bucketName)).Scan(
			&config.ConfigID,
			&config.TopicName,
			&spannerEvents,
			&config.FilterPrefix,
			&config.FilterSuffix,
			&config.CreatedAt,
			&config.UpdatedAt,
		)

		// Convert []spanner.NullString to []string
		if err == nil {
			config.Events = make([]string, len(spannerEvents))
			for i, ns := range spannerEvents {
				config.Events[i] = ns.StringVal
			}
		}

	default:
		return nil, buckets.ErrBucket.New("unsupported database implementation: %v", db.db.impl)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, buckets.ErrBucket.Wrap(err)
	}

	return &config, nil
}

// UpdateBucketNotificationConfig updates the bucket notification configuration for a bucket.
func (db *bucketsDB) UpdateBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID, config buckets.NotificationConfig) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Validate required fields
	if config.TopicName == "" {
		return buckets.ErrBucket.New("notification configuration must have a topic name")
	}
	if len(config.Events) == 0 {
		return buckets.ErrBucket.New("notification configuration must have at least one event")
	}

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		if config.ConfigID == "" {
			// When config_id is empty, omit it from INSERT to trigger DEFAULT,
			// and preserve existing value on UPDATE
			_, err = db.db.ExecContext(ctx, `
				INSERT INTO bucket_eventing_configs (
					project_id, bucket_name, topic_name, events, filter_prefix, filter_suffix
				) VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (project_id, bucket_name)
				DO UPDATE SET
					topic_name = EXCLUDED.topic_name,
					events = EXCLUDED.events,
					filter_prefix = EXCLUDED.filter_prefix,
					filter_suffix = EXCLUDED.filter_suffix,
					updated_at = CURRENT_TIMESTAMP
			`, projectID[:], bucketName, config.TopicName, pgutil.TextArray(config.Events), config.FilterPrefix, config.FilterSuffix)
		} else {
			// When config_id is provided, include it in both INSERT and UPDATE
			_, err = db.db.ExecContext(ctx, `
				INSERT INTO bucket_eventing_configs (
					project_id, bucket_name, config_id, topic_name, events, filter_prefix, filter_suffix
				) VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (project_id, bucket_name)
				DO UPDATE SET
					config_id = EXCLUDED.config_id,
					topic_name = EXCLUDED.topic_name,
					events = EXCLUDED.events,
					filter_prefix = EXCLUDED.filter_prefix,
					filter_suffix = EXCLUDED.filter_suffix,
					updated_at = CURRENT_TIMESTAMP
			`, projectID[:], bucketName, config.ConfigID, config.TopicName, pgutil.TextArray(config.Events), config.FilterPrefix, config.FilterSuffix)
		}

		return buckets.ErrBucket.Wrap(err)

	case dbutil.Spanner:
		return spannerutil.UnderlyingClient(ctx, db.db, func(client *spanner.Client) error {
			var mutation *spanner.Mutation

			columns := []string{
				"project_id", "bucket_name", "topic_name", "events",
				"filter_prefix", "filter_suffix", "updated_at",
			}
			values := []any{
				projectID.Bytes(), bucketName, config.TopicName, config.Events,
				config.FilterPrefix, config.FilterSuffix, spanner.CommitTimestamp,
			}

			if config.ConfigID != "" {
				columns = append(columns, "config_id")
				values = append(values, config.ConfigID)
			}

			mutation = spanner.InsertOrUpdate("bucket_eventing_configs", columns, values)

			_, err := client.Apply(ctx, []*spanner.Mutation{mutation})
			return buckets.ErrBucket.Wrap(err)
		})

	default:
		return buckets.ErrBucket.New("unsupported database implementation: %v", db.db.impl)
	}
}

// DeleteBucketNotificationConfig removes the notification configuration for a bucket.
func (db *bucketsDB) DeleteBucketNotificationConfig(ctx context.Context, bucketName []byte, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		_, err = db.db.ExecContext(ctx, `
			DELETE FROM bucket_eventing_configs
			WHERE project_id = $1 AND bucket_name = $2
		`, projectID[:], bucketName)

	case dbutil.Spanner:
		_, err = db.db.ExecContext(ctx, `
			DELETE FROM bucket_eventing_configs
			WHERE project_id = @project_id AND bucket_name = @bucket_name
		`, sql.Named("project_id", projectID.Bytes()), sql.Named("bucket_name", bucketName))

	default:
		return buckets.ErrBucket.New("unsupported database implementation: %v", db.db.impl)
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return buckets.ErrBucket.Wrap(err)
	}

	// No error if configuration doesn't exist
	return nil
}
