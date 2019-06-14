// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/valueattribution"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type valueattributionDB struct {
	db *dbx.DB
}

// Get reads the partner info
func (keys *valueattributionDB) Get(ctx context.Context, bucketname []byte) (info *valueattribution.PartnerInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Get_ValueAttribution_By_BucketName(ctx, dbx.ValueAttribution_BucketName(bucketname))
	if err != nil {
		return nil, err
	}

	return &valueattribution.PartnerInfo{
		PartnerID:  dbxInfo.PartnerId,
		BucketName: dbxInfo.BucketName,
		CreatedAt:  dbxInfo.LastUpdated,
	}, nil
}

// Insert implements create partner info
func (keys *valueattributionDB) Insert(ctx context.Context, partnerinfo *valueattribution.PartnerInfo) (info *valueattribution.PartnerInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Create_ValueAttribution(ctx, dbx.ValueAttribution_ProjectId(partnerinfo.BucketName[:16]),
		dbx.ValueAttribution_BucketName(partnerinfo.BucketName), dbx.ValueAttribution_PartnerId(partnerinfo.PartnerID))
	if err != nil {
		return nil, err
	}

	return &valueattribution.PartnerInfo{
		PartnerID:  dbxInfo.PartnerId,
		BucketName: dbxInfo.BucketName,
		CreatedAt:  dbxInfo.LastUpdated,
	}, nil
}
