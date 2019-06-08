// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// connectorkeys is an implementation of satellite.APIKeys
type connectorkeys struct {
	db *dbx.DB
}

// GetByProjectID
func (keys *connectorkeys) GetByProjectID(ctx context.Context, projectID uuid.UUID) (_ *console.ConnectorKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Get_ValueAttribution_By_BucketId(ctx, dbx.ValueAttribution_BucketId(projectID[:]))
	if err != nil {
		return nil, err
	}

	return &console.ConnectorKeyInfo{
		PartnerID: dbxInfo.PartnerId,
		BucketID:  dbxInfo.BucketId,
		CreatedAt: dbxInfo.LastUpdated,
	}, nil
}

// Create implements satellite.APIKeys
func (keys *connectorkeys) Create(ctx context.Context, info console.ConnectorKeyInfo) (connectorkeyinfo *console.ConnectorKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := keys.db.Create_ValueAttribution(ctx, dbx.ValueAttribution_BucketId(info.BucketID),
		dbx.ValueAttribution_PartnerId(info.PartnerID), dbx.ValueAttribution_LastUpdated(info.CreatedAt))
	if err != nil {
		return nil, err
	}
	return &console.ConnectorKeyInfo{
		PartnerID: dbxInfo.PartnerId,
		BucketID:  dbxInfo.BucketId,
		CreatedAt: dbxInfo.LastUpdated,
	}, nil
}

// Delete implements satellite.APIKeys
func (keys *connectorkeys) Delete(ctx context.Context, id uuid.UUID) (err error) {
	return nil
}
