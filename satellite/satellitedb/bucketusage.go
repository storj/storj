package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bucketusages struct {
	db dbx.Methods
}

// Get retrieves bucket usage rollup info by id
func (bucketusages *bucketusages) Get(ctx context.Context, id uuid.UUID) (*console.BucketUsage, error) {
	dbxUsage, err := bucketusages.db.Get_BucketUsage_By_Id(ctx, dbx.BucketUsage_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXUsage(dbxUsage)
}

// GetByBucketID retrieves list of bucket usage rollup entries
func (bucketusages *bucketusages) GetByBucketID(ctx context.Context, iterator *console.UsageIterator) ([]console.BucketUsage, error) {
	var getUsage func(context.Context,
		dbx.BucketUsage_BucketId_Field,
		dbx.BucketUsage_RollupEndTime_Field,
		dbx.BucketUsage_Id_Field, int, int64) ([]*dbx.BucketUsage, error)

	switch iterator.Order {
	case console.Desc:
		getUsage = bucketusages.db.Limited_BucketUsage_By_BucketId_And_RollupEndTime_Greater_And_Id_Less_OrderBy_Desc_Id
	default:
		getUsage = bucketusages.db.Limited_BucketUsage_By_BucketId_And_RollupEndTime_Greater_And_Id_Greater_OrderBy_Asc_Id
	}

	dbxUsages, err := getUsage(
		ctx,
		dbx.BucketUsage_BucketId(iterator.BucketID[:]),
		dbx.BucketUsage_RollupEndTime(iterator.After),
		dbx.BucketUsage_Id(iterator.Cursor[:]),
		iterator.Limit,
		0,
	)

	if err != nil {
		return nil, err
	}

	var usages []console.BucketUsage
	for _, dbxUsage := range dbxUsages {
		usage, err := fromDBXUsage(dbxUsage)
		if err != nil {
			return nil, err
		}

		usages = append(usages, *usage)
	}

	size := len(usages)
	if size == iterator.Limit {
		iterator.Next = &console.UsageIterator{
			BucketID: iterator.BucketID,
			Cursor:   usages[size-1].ID,
			Order:    iterator.Order,
			After:    iterator.After,
			Limit:    iterator.Limit,
		}
	}
	return usages, nil
}

// Create creates new bucket usage rollup
func (bucketusages *bucketusages) Create(ctx context.Context, usage console.BucketUsage) (*console.BucketUsage, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	dbxUsage, err := bucketusages.db.Create_BucketUsage(
		ctx,
		dbx.BucketUsage_Id(id[:]),
		dbx.BucketUsage_BucketId(usage.BucketID[:]),
		dbx.BucketUsage_RollupEndTime(usage.RollupEndTime),
		dbx.BucketUsage_RemoteStoredData(usage.RemoteStoredData),
		dbx.BucketUsage_InlineStoredData(usage.InlineStoredData),
		dbx.BucketUsage_Segments(usage.Segments),
		dbx.BucketUsage_MetadataSize(usage.MetadataSize),
		dbx.BucketUsage_RepairEgress(usage.RepairEgress),
		dbx.BucketUsage_GetEgress(usage.GetEgress),
		dbx.BucketUsage_AuditEgress(usage.AuditEgress),
	)

	if err != nil {
		return nil, err
	}

	return fromDBXUsage(dbxUsage)
}

// Delete deletes bucket usage rollup entry by id
func (bucketusages *bucketusages) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := bucketusages.db.Delete_BucketUsage_By_Id(ctx, dbx.BucketUsage_Id(id[:]))
	return err
}

// fromDBXUsage helper method to conert dbx.BucketUsage to console.BucketUsage
func fromDBXUsage(dbxUsage *dbx.BucketUsage) (*console.BucketUsage, error) {
	id, err := bytesToUUID(dbxUsage.Id)
	if err != nil {
		return nil, err
	}

	bucketID, err := bytesToUUID(dbxUsage.BucketId)
	if err != nil {
		return nil, err
	}

	return &console.BucketUsage{
		ID:               id,
		BucketID:         bucketID,
		RollupEndTime:    dbxUsage.RollupEndTime,
		RemoteStoredData: dbxUsage.RemoteStoredData,
		InlineStoredData: dbxUsage.InlineStoredData,
		Segments:         dbxUsage.Segments,
		MetadataSize:     dbxUsage.MetadataSize,
		RepairEgress:     dbxUsage.RepairEgress,
		GetEgress:        dbxUsage.GetEgress,
		AuditEgress:      dbxUsage.AuditEgress,
	}, nil
}
