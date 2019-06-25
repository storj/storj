// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/attribution"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type attributionDB struct {
	db *dbx.DB
}

// GetByBucket retrieves attribution info using bucket name.
func (keys *attributionDB) GetByBucket(ctx context.Context, bucketName []byte) (info *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Get_ValueAttribution_By_BucketName(ctx, dbx.ValueAttribution_BucketName(bucketName))
	if err == sql.ErrNoRows {
		return nil, attribution.ErrBucketNotAttributed.New(string(bucketName))
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return attributionFromDBX(dbxInfo)
}

// Get reads the partner info
func (keys *attributionDB) Get(ctx context.Context, projectID uuid.UUID, bucketName []byte) (info *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Get_ValueAttribution_By_ProjectId_And_BucketName(ctx,
		dbx.ValueAttribution_ProjectId(projectID[:]),
		dbx.ValueAttribution_BucketName(bucketName),
	)
	if err == sql.ErrNoRows {
		return nil, attribution.ErrBucketNotAttributed.New(string(bucketName))
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return attributionFromDBX(dbxInfo)
}

// Insert implements create partner info
func (keys *attributionDB) Insert(ctx context.Context, info *attribution.Info) (_ *attribution.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Create_ValueAttribution(ctx,
		dbx.ValueAttribution_ProjectId(info.ProjectID[:]),
		dbx.ValueAttribution_BucketName(info.BucketName),
		dbx.ValueAttribution_PartnerId(info.PartnerID[:]),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return attributionFromDBX(dbxInfo)
}

func attributionFromDBX(info *dbx.ValueAttribution) (*attribution.Info, error) {
	partnerID, err := bytesToUUID(info.PartnerId)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	projectID, err := bytesToUUID(info.ProjectId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &attribution.Info{
		ProjectID:  projectID,
		BucketName: info.BucketName,
		PartnerID:  partnerID,
		CreatedAt:  info.LastUpdated,
	}, nil
}
