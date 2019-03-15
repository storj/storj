// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/storj"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// bucketBandwidthUsage exposes methods to manage bucketBandwidthUsage table in the database.
type bucketBandwidthUsage struct {
	db *dbx.DB
}

// Create a new bucketBandwidthUsage that stores bwagreement info for a bucket.
func (usage *bucketBandwidthUsage) Create(ctx context.Context, pba *pb.OrderLimit, path storj.Path) error {
	id, err := uuid.New()
	if err != nil {
		return err
	}

	pathElements := storj.SplitPath(path)
	projectID, bucketID := pathElements[0], pathElements[2]

	_, err = usage.db.Create_BucketBandwidthUsage(
		ctx,
		dbx.BucketBandwidthUsage_Id(id[:]),
		dbx.BucketBandwidthUsage_Serialnum(pba.GetSerialNumber()),
		dbx.BucketBandwidthUsage_BucketId([]byte(bucketID)),
		dbx.BucketBandwidthUsage_ProjectId([]byte(projectID)),
		dbx.BucketBandwidthUsage_Action(0),
		// FIXME: pba.GetAction() is type err
		// dbx.BucketBandwidthUsage_Action(pba.GetAction()),
		dbx.BucketBandwidthUsage_Total(pba.GetMaxSize()),
	)
	if err != nil {
		return err
	}
	return nil
}

// DeleteByByBucketID returns all BucketBandwidthUsage records with the Bucket ID and Action.
func (usage *bucketBandwidthUsage) DeleteByBucketID(ctx context.Context, bucketID string) error {
	// TODO: implement
	return nil
}

// GetAllByBucketIDAndAction returns all BucketBandwidthUsage records with the Bucket ID and Action.
func (usage *bucketBandwidthUsage) GetAllByBucketIDAndAction(ctx context.Context, bucketID string, action pb.BandwidthAction) ([]accounting.BucketBWUsage, error) {
	rows, err := usage.db.All_BucketBandwidthUsage_By_BucketId_And_Action(
		ctx,
		dbx.BucketBandwidthUsage_BucketId([]byte(bucketID)),
		dbx.BucketBandwidthUsage_Action(int64(action)),
	)
	if err != nil {
		return nil, err
	}

	results := []accounting.BucketBWUsage{}
	for _, row := range rows {
		id, err := bytesToUUID(row.Id)
		if err != nil {
			return nil, err
		}
		usage := accounting.BucketBWUsage{
			ID:        id,
			Serialnum: row.Serialnum,
			BucketID:  string(row.BucketId),
			ProjectID: string(row.ProjectId),
			Action:    row.Action,
			Total:     row.Total,
			CreatedAt: row.CreatedAt,
		}
		results = append(results, usage)
	}
	return results, nil
}
