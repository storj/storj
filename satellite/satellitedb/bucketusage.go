// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/accounting"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bucketusage struct {
	db dbx.Methods
}

// Get retrieves bucket usage tally info by id
func (usage *bucketusage) Get(ctx context.Context, id uuid.UUID) (*accounting.BucketTally, error) {
	dbxUsage, err := usage.db.Get_BucketUsage_By_Id(ctx, dbx.BucketUsage_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXUsage(dbxUsage)
}

// GetPaged retrieves list of bucket usage tally entries for given cursor
func (usage bucketusage) GetPaged(ctx context.Context, cursor *accounting.BucketTallyCursor) ([]accounting.BucketTally, error) {
	var getUsage func(context.Context,
		dbx.BucketUsage_BucketId_Field,
		dbx.BucketUsage_TallyEndTime_Field,
		dbx.BucketUsage_TallyEndTime_Field,
		int, int64) ([]*dbx.BucketUsage, error)

	switch cursor.Order {
	case accounting.Desc:
		getUsage = usage.db.Limited_BucketUsage_By_BucketId_And_TallyEndTime_Greater_And_TallyEndTime_LessOrEqual_OrderBy_Desc_TallyEndTime
	default:
		getUsage = usage.db.Limited_BucketUsage_By_BucketId_And_TallyEndTime_Greater_And_TallyEndTime_LessOrEqual_OrderBy_Asc_TallyEndTime
	}

	dbxUsages, err := getUsage(
		ctx,
		dbx.BucketUsage_BucketId(cursor.BucketID[:]),
		dbx.BucketUsage_TallyEndTime(cursor.After),
		dbx.BucketUsage_TallyEndTime(cursor.Before),
		cursor.PageSize,
		0,
	)

	if err != nil {
		return nil, err
	}

	var tallies []accounting.BucketTally
	for _, dbxUsage := range dbxUsages {
		tally, err := fromDBXUsage(dbxUsage)
		if err != nil {
			return nil, err
		}

		tallies = append(tallies, *tally)
	}

	switch cursor.Order {
	// going backwards
	case accounting.Desc:
		dbxUsages, err := getUsage(
			ctx,
			dbx.BucketUsage_BucketId(cursor.BucketID[:]),
			dbx.BucketUsage_TallyEndTime(cursor.After),
			dbx.BucketUsage_TallyEndTime(tallies[len(tallies)-1].TallyEndTime),
			2,
			0,
		)

		if err != nil {
			return nil, err
		}

		if len(dbxUsages) == 2 {
			cursor.Next = &accounting.BucketTallyCursor{
				BucketID: cursor.BucketID,
				After:    cursor.After,
				Before:   dbxUsages[1].TallyEndTime,
				Order:    cursor.Order,
				PageSize: cursor.PageSize,
			}
		}
	// going forward
	default:
		dbxUsages, err := getUsage(
			ctx,
			dbx.BucketUsage_BucketId(cursor.BucketID[:]),
			dbx.BucketUsage_TallyEndTime(tallies[len(tallies)-1].TallyEndTime),
			dbx.BucketUsage_TallyEndTime(cursor.Before),
			1,
			0,
		)

		if err != nil {
			return nil, err
		}

		if len(dbxUsages) > 0 {
			cursor.Next = &accounting.BucketTallyCursor{
				BucketID: cursor.BucketID,
				After:    tallies[len(tallies)-1].TallyEndTime,
				Before:   cursor.Before,
				Order:    cursor.Order,
				PageSize: cursor.PageSize,
			}
		}
	}

	return tallies, nil
}

// Create creates new bucket usage tally
func (usage bucketusage) Create(ctx context.Context, tally accounting.BucketTally) (*accounting.BucketTally, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	dbxUsage, err := usage.db.Create_BucketUsage(
		ctx,
		dbx.BucketUsage_Id(id[:]),
		dbx.BucketUsage_BucketId(tally.BucketID[:]),
		dbx.BucketUsage_TallyEndTime(tally.TallyEndTime),
		dbx.BucketUsage_RemoteStoredData(tally.RemoteStoredData),
		dbx.BucketUsage_InlineStoredData(tally.InlineStoredData),
		dbx.BucketUsage_RemoteSegments(tally.RemoteSegments),
		dbx.BucketUsage_InlineSegments(tally.InlineSegments),
		dbx.BucketUsage_Objects(tally.Objects),
		dbx.BucketUsage_MetadataSize(tally.MetadataSize),
		dbx.BucketUsage_RepairEgress(tally.RepairEgress),
		dbx.BucketUsage_GetEgress(tally.GetEgress),
		dbx.BucketUsage_AuditEgress(tally.AuditEgress),
	)

	if err != nil {
		return nil, err
	}

	return fromDBXUsage(dbxUsage)
}

// Delete deletes bucket usage tally entry by id
func (usage *bucketusage) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := usage.db.Delete_BucketUsage_By_Id(ctx, dbx.BucketUsage_Id(id[:]))
	return err
}

// fromDBXUsage helper method to conert dbx.BucketUsage to accounting.BucketTally
func fromDBXUsage(dbxUsage *dbx.BucketUsage) (*accounting.BucketTally, error) {
	id, err := bytesToUUID(dbxUsage.Id)
	if err != nil {
		return nil, err
	}

	bucketID, err := bytesToUUID(dbxUsage.BucketId)
	if err != nil {
		return nil, err
	}

	return &accounting.BucketTally{
		ID:               id,
		BucketID:         bucketID,
		TallyEndTime:    dbxUsage.TallyEndTime,
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
