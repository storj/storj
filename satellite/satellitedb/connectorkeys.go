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
	db dbx.Methods
}

// GetByProjectID
func (keys *connectorkeys) GetByProjectID(ctx context.Context, projectID uuid.UUID) (connectorkeyinfo *console.ConnectorKeyInfo, err error) {

	return connectorkeyinfo, nil
}

// Create implements satellite.APIKeys
func (keys *connectorkeys) Create(ctx context.Context, head []byte, info console.APIKeyInfo) (connectorkeyinfo *console.ConnectorKeyInfo, err error) {
	return connectorkeyinfo, nil
}

// Delete implements satellite.APIKeys
func (keys *connectorkeys) Delete(ctx context.Context, id uuid.UUID) (err error) {
	return nil
}
