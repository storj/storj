// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

// apikeys is an implementation of satellite.APIKeys
type apikeys struct {
	db dbx.Methods
}

// GetByProjectID implements satellite.APIKeys ordered by name
func (keys *apikeys) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]satellite.APIKey, error) {
	dbKeys, err := keys.db.All_ApiKey_By_ProjectId_OrderBy_Asc_Name(ctx, dbx.ApiKey_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	var apiKeys []satellite.APIKey
	var parseErr errs.Group

	for _, key := range dbKeys {
		apiKey, err := toAPIKey(key)
		if err != nil {
			parseErr.Add(err)
			continue
		}

		apiKeys = append(apiKeys, *apiKey)
	}

	if err := parseErr.Err(); err != nil {
		return nil, err
	}

	return apiKeys, nil
}

// Get implements satellite.APIKeys
func (keys *apikeys) Get(ctx context.Context, id uuid.UUID) (*satellite.APIKey, error) {
	dbKey, err := keys.db.Get_ApiKey_By_Id(ctx, dbx.ApiKey_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return toAPIKey(dbKey)
}

// Create implements satellite.APIKeys
func (keys *apikeys) Create(ctx context.Context, key satellite.APIKey) (*satellite.APIKey, error) {
	id, err := uuid.New()
	if err != nil {
		return nil, err
	}

	dbKey, err := keys.db.Create_ApiKey(
		ctx,
		dbx.ApiKey_Id(id[:]),
		dbx.ApiKey_ProjectId(key.ProjectID[:]),
		dbx.ApiKey_Key(key.Key[:]),
		dbx.ApiKey_Name(key.Name),
	)

	if err != nil {
		return nil, err
	}

	return toAPIKey(dbKey)
}

// Update implements satellite.APIKeys
func (keys *apikeys) Update(ctx context.Context, key satellite.APIKey) error {
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

// toAPIKey converts dbx.ApiKey to satellite.APIKey
func toAPIKey(key *dbx.ApiKey) (*satellite.APIKey, error) {
	id, err := bytesToUUID(key.Id)
	if err != nil {
		return nil, err
	}

	projectID, err := bytesToUUID(key.ProjectId)
	if err != nil {
		return nil, err
	}

	return &satellite.APIKey{
		APIKeyInfo: satellite.APIKeyInfo{
			ID:        id,
			ProjectID: projectID,
			Name:      key.Name,
			CreatedAt: key.CreatedAt,
		},
		Key: *satellite.MockKeyFromBytes(key.Key),
	}, nil
}
