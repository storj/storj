// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/accounting"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bucketusage struct {
	db dbx.Methods
}

// Get retrieves bucket usage rollup info by id
func (usage *bucketusage) Get(ctx context.Context, id uuid.UUID) (_ *accounting.BucketRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxUsage, err := usage.db.Get_BucketUsage_By_Id(ctx, dbx.BucketUsage_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXUsage(ctx, dbxUsage)
}

// GetPaged retrieves list of bucket usage rollup entries for given cursor
func (usage bucketusage) GetPaged(ctx context.Context, cursor *accounting.BucketRollupCursor) (_ []accounting.BucketRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	var getUsage func(context.Context,
		dbx.BucketUsage_BucketId_Field,
		dbx.BucketUsage_RollupEndTime_Field,
		dbx.BucketUsage_RollupEndTime_Field,
		int, int64) ([]*dbx.BucketUsage, error)

	switch cursor.Order {
	case accounting.Desc:
		getUsage = usage.db.Limited_BucketUsage_By_BucketId_And_RollupEndTime_Greater_And_RollupEndTime_LessOrEqual_OrderBy_Desc_RollupEndTime
	default:
		getUsage = usage.db.Limited_BucketUsage_By_BucketId_And_RollupEndTime_Greater_And_RollupEndTime_LessOrEqual_OrderBy_Asc_RollupEndTime
	}

	dbxUsages, err := getUsage(
		ctx,
		dbx.BucketUsage_BucketId(cursor.BucketID[:]),
		dbx.BucketUsage_RollupEndTime(cursor.After),
		dbx.BucketUsage_RollupEndTime(cursor.Before),
		cursor.PageSize,
		0,
	)

	if err != nil {
		return nil, err
	}

	var rollups []accounting.BucketRollup
	for _, dbxUsage := range dbxUsages {
		rollup, err := fromDBXUsage(ctx, dbxUsage)
		if err != nil {
			return nil, err
		}

		rollups = append(rollups, *rollup)
	}

	switch cursor.Order {
	// going backwards
	case accounting.Desc:
		dbxUsages, err := getUsage(
			ctx,
			dbx.BucketUsage_BucketId(cursor.BucketID[:]),
			dbx.BucketUsage_RollupEndTime(cursor.After),
			dbx.BucketUsage_RollupEndTime(rollups[len(rollups)-1].RollupEndTime),
			2,
			0,
		)

		if err != nil {
			return nil, err
		}

		if len(dbxUsages) == 2 {
			cursor.Next = &accounting.BucketRollupCursor{
				BucketID: cursor.BucketID,
				After:    cursor.After,
				Before:   dbxUsages[1].RollupEndTime,
				Order:    cursor.Order,
				PageSize: cursor.PageSize,
			}
		}
	// going forward
	default:
		dbxUsages, err := getUsage(
			ctx,
			dbx.BucketUsage_BucketId(cursor.BucketID[:]),
			dbx.BucketUsage_RollupEndTime(rollups[len(rollups)-1].RollupEndTime),
			dbx.BucketUsage_RollupEndTime(cursor.Before),
			1,
			0,
		)

		if err != nil {
			return nil, err
		}

		if len(dbxUsages) > 0 {
			cursor.Next = &accounting.BucketRollupCursor{
				BucketID: cursor.BucketID,
				After:    rollups[len(rollups)-1].RollupEndTime,
				Before:   cursor.Before,
				Order:    cursor.Order,
				PageSize: cursor.PageSize,
			}
		}
	}

	return rollups, nil
}

// Create creates new bucket usage rollup
func (usage bucketusage) Create(ctx context.Context, rollup accounting.BucketRollup) (_ *accounting.BucketRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	dbxUsage, err := usage.db.Create_BucketUsage(
		ctx,
		dbx.BucketUsage_Id(id[:]),
		dbx.BucketUsage_BucketId(rollup.BucketID[:]),
		dbx.BucketUsage_RollupEndTime(rollup.RollupEndTime),
		dbx.BucketUsage_RemoteStoredData(rollup.RemoteStoredData),
		dbx.BucketUsage_InlineStoredData(rollup.InlineStoredData),
		dbx.BucketUsage_RemoteSegments(rollup.RemoteSegments),
		dbx.BucketUsage_InlineSegments(rollup.InlineSegments),
		dbx.BucketUsage_Objects(rollup.Objects),
		dbx.BucketUsage_MetadataSize(rollup.MetadataSize),
		dbx.BucketUsage_RepairEgress(rollup.RepairEgress),
		dbx.BucketUsage_GetEgress(rollup.GetEgress),
		dbx.BucketUsage_AuditEgress(rollup.AuditEgress),
	)

	if err != nil {
		return nil, err
	}

	return fromDBXUsage(ctx, dbxUsage)
}

// Delete deletes bucket usage rollup entry by id
func (usage *bucketusage) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = usage.db.Delete_BucketUsage_By_Id(ctx, dbx.BucketUsage_Id(id[:]))
	return err
}

// fromDBXUsage helper method to conert dbx.BucketUsage to accounting.BucketRollup
func fromDBXUsage(ctx context.Context, dbxUsage *dbx.BucketUsage) (_ *accounting.BucketRollup, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := bytesToUUID(dbxUsage.Id)
	if err != nil {
		return nil, err
	}

	bucketID, err := bytesToUUID(dbxUsage.BucketId)
	if err != nil {
		return nil, err
	}

	return &accounting.BucketRollup{
		ID:               id,
		BucketID:         bucketID,
		RollupEndTime:    dbxUsage.RollupEndTime,
		RemoteStoredData: dbxUsage.RemoteStoredData,
		InlineStoredData: dbxUsage.InlineStoredData,
		RemoteSegments:   dbxUsage.RemoteSegments,
		InlineSegments:   dbxUsage.InlineSegments,
		Objects:          dbxUsage.Objects,
		MetadataSize:     dbxUsage.MetadataSize,
		RepairEgress:     dbxUsage.RepairEgress,
		GetEgress:        dbxUsage.GetEgress,
		AuditEgress:      dbxUsage.AuditEgress,
	}, nil
}
