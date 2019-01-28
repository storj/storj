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
func (keys *apikeys) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]console.APIKeyInfo, error) {
	dbKeys, err := keys.db.All_ApiKey_By_ProjectId_OrderBy_Asc_Name(ctx, dbx.ApiKey_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	var apiKeys []console.APIKeyInfo
	var parseErr errs.Group

	for _, key := range dbKeys {
		info, err := fromDBXAPIKey(key)
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
func (keys *apikeys) Get(ctx context.Context, id uuid.UUID) (*console.APIKeyInfo, error) {
	dbKey, err := keys.db.Get_ApiKey_By_Id(ctx, dbx.ApiKey_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(dbKey)
}

// GetByKey implements satellite.APIKeys
func (keys *apikeys) GetByKey(ctx context.Context, key console.APIKey) (*console.APIKeyInfo, error) {
	dbKey, err := keys.db.Get_ApiKey_By_Key(ctx, dbx.ApiKey_Key(key[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(dbKey)
}

// Create implements satellite.APIKeys
func (keys *apikeys) Create(ctx context.Context, key console.APIKey, info console.APIKeyInfo) (*console.APIKeyInfo, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	dbKey, err := keys.db.Create_ApiKey(
		ctx,
		dbx.ApiKey_Id(id[:]),
		dbx.ApiKey_ProjectId(info.ProjectID[:]),
		dbx.ApiKey_Key(key[:]),
		dbx.ApiKey_Name(info.Name),
	)

	if err != nil {
		return nil, err
	}

	return fromDBXAPIKey(dbKey)
}

// Update implements satellite.APIKeys
func (keys *apikeys) Update(ctx context.Context, key console.APIKeyInfo) error {
	_, err := keys.db.Update_ApiKey_By_Id(
		ctx,
		dbx.ApiKey_Id(key.ID[:]),
		dbx.ApiKey_Update_Fields{
			Name: dbx.ApiKey_Name(key.Name),
		},
	)

	return err
}

// Delete implements satellite.APIKeys
func (keys *apikeys) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := keys.db.Delete_ApiKey_By_Id(ctx, dbx.ApiKey_Id(id[:]))
	return err
}

// fromDBXAPIKey converts dbx.ApiKey to satellite.APIKeyInfo
func fromDBXAPIKey(key *dbx.ApiKey) (*console.APIKeyInfo, error) {
	id, err := bytesToUUID(key.Id)
	if err != nil {
		return nil, err
	}

	projectID, err := bytesToUUID(key.ProjectId)
	if err != nil {
		return nil, err
	}

	return &console.APIKeyInfo{
		ID:        id,
		ProjectID: projectID,
		Name:      key.Name,
		CreatedAt: key.CreatedAt,
	}, nil
}
