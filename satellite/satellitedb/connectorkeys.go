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
	return fromDBXConnectorKey(ctx, dbxInfo)
}

// Create implements satellite.APIKeys
func (keys *connectorkeys) Create(ctx context.Context, info console.ConnectorKeyInfo) (connectorkeyinfo *console.ConnectorKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := keys.db.Create_ValueAttribution(ctx, dbx.ValueAttribution_BucketId(info.ProjectID[:]),
		dbx.ValueAttribution_PartnerId(info.ID[:]), dbx.ValueAttribution_LastUpdated(info.CreatedAt))
	return fromDBXConnectorKey(ctx, dbxInfo)
}

// Delete implements satellite.APIKeys
func (keys *connectorkeys) Delete(ctx context.Context, id uuid.UUID) (err error) {
	return nil
}

// fromDBXConnectorKey converts dbx.ValueAttribution to connectory key info
func fromDBXConnectorKey(ctx context.Context, key *dbx.ValueAttribution) (_ *console.ConnectorKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := bytesToUUID(key.PartnerId)
	if err != nil {
		return nil, err
	}

	projectID, err := bytesToUUID(key.BucketId)
	if err != nil {
		return nil, err
	}

	return &console.ConnectorKeyInfo{
		ID:        id,
		ProjectID: projectID,
		CreatedAt: key.LastUpdated,
	}, nil
}
