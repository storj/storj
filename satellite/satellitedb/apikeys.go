// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// apikeys is an implementation of satellite.APIKeys
type apikeys struct {
	db dbx.Methods
}

// GetByProjectID implements satellite.APIKeys ordered by name
func (keys *apikeys) GetByProjectID(ctx context.Context, projectID uuid.UUID) (_ []console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbKeys, err := keys.db.All_ApiKey_By_ProjectId_OrderBy_Asc_Name(ctx, dbx.ApiKey_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	var apiKeys []console.APIKeyInfo
	var parseErr errs.Group

	for _, key := range dbKeys {
		info, err := fromDBXAPIKey(ctx, key)
		if err != nil {
			parseErr.Add(err)
			continue
		}

		apiKeys = append(apiKeys, *info)
	}

	if err := parseErr.Err(); err != nil {
		return nil, err
	}

	return apiKeys, nil
}

// Get implements satellite.APIKeys
func (keys *apikeys) Get(ctx context.Context, id uuid.UUID) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbKey, err := keys.db.Get_ApiKey_By_Id(ctx, dbx.ApiKey_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(ctx, dbKey)
}

// GetByHead implements satellite.APIKeys
func (keys *apikeys) GetByHead(ctx context.Context, head []byte) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	dbKey, err := keys.db.Get_ApiKey_By_Head(ctx, dbx.ApiKey_Head(head))
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(ctx, dbKey)
}

// Create implements satellite.APIKeys
func (keys *apikeys) Create(ctx context.Context, head []byte, info console.APIKeyInfo) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	optional := dbx.ApiKey_Create_Fields{}
	if !info.PartnerID.IsZero() {
		optional.PartnerId = dbx.ApiKey_PartnerId(info.PartnerID[:])
	}

	dbKey, err := keys.db.Create_ApiKey(
		ctx,
		dbx.ApiKey_Id(id[:]),
		dbx.ApiKey_ProjectId(info.ProjectID[:]),
		dbx.ApiKey_Head(head),
		dbx.ApiKey_Name(info.Name),
		dbx.ApiKey_Secret(info.Secret),
		optional,
	)

	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(ctx, dbKey)
}

// Update implements satellite.APIKeys
func (keys *apikeys) Update(ctx context.Context, key console.APIKeyInfo) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = keys.db.Update_ApiKey_By_Id(
		ctx,
		dbx.ApiKey_Id(key.ID[:]),
		dbx.ApiKey_Update_Fields{
			Name: dbx.ApiKey_Name(key.Name),
		},
	)

	return err
}

// Delete implements satellite.APIKeys
func (keys *apikeys) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = keys.db.Delete_ApiKey_By_Id(ctx, dbx.ApiKey_Id(id[:]))
	return err
}

// fromDBXAPIKey converts dbx.ApiKey to satellite.APIKeyInfo
func fromDBXAPIKey(ctx context.Context, key *dbx.ApiKey) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := bytesToUUID(key.Id)
	if err != nil {
		return nil, err
	}

	projectID, err := bytesToUUID(key.ProjectId)
	if err != nil {
		return nil, err
	}

	result := &console.APIKeyInfo{
		ID:        id,
		ProjectID: projectID,
		Name:      key.Name,
		CreatedAt: key.CreatedAt,
		Secret:    key.Secret,
	}

	if key.PartnerId != nil {
		result.PartnerID, err = bytesToUUID(key.PartnerId)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
