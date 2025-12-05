// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"database/sql"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type restApiKeysDB struct {
	db dbx.DriverMethods
}

// Get retrieves the RestAPIKey for the given ID.
func (o *restApiKeysDB) Get(ctx context.Context, id uuid.UUID) (key restapikeys.Key, err error) {
	defer mon.Task()(&ctx)(&err)

	dbKey, err := o.db.Get_RestApiKey_By_Id(ctx, dbx.RestApiKey_Id(id[:]))
	if err != nil {
		return key, err
	}

	ID, err := uuid.FromBytes(dbKey.Id)
	if err != nil {
		return key, err
	}

	userID, err := uuid.FromBytes(dbKey.UserId)
	if err != nil {
		return key, err
	}

	if dbKey.ExpiresAt != nil && time.Now().After(*dbKey.ExpiresAt) {
		return restapikeys.Key{}, sql.ErrNoRows
	}

	key.ID = ID
	key.UserID = userID
	key.Name = dbKey.Name
	key.Token = string(dbKey.Token)
	key.ExpiresAt = dbKey.ExpiresAt
	key.CreatedAt = dbKey.CreatedAt

	return key, nil
}

// GetByToken retrieves the RestAPIKey by the given Token.
func (o *restApiKeysDB) GetByToken(ctx context.Context, token string) (key restapikeys.Key, err error) {
	defer mon.Task()(&ctx)(&err)

	dbKey, err := o.db.Get_RestApiKey_By_Token(ctx, dbx.RestApiKey_Token([]byte(token)))
	if err != nil {
		return key, err
	}

	ID, err := uuid.FromBytes(dbKey.Id)
	if err != nil {
		return key, err
	}

	userID, err := uuid.FromBytes(dbKey.UserId)
	if err != nil {
		return key, err
	}

	if dbKey.ExpiresAt != nil && time.Now().After(*dbKey.ExpiresAt) {
		return restapikeys.Key{}, sql.ErrNoRows
	}

	key.ID = ID
	key.UserID = userID
	key.Name = dbKey.Name
	key.Token = string(dbKey.Token)
	key.ExpiresAt = dbKey.ExpiresAt
	key.CreatedAt = dbKey.CreatedAt

	return key, nil
}

// GetAll gets a list of REST API keys for the provided user.
func (o *restApiKeysDB) GetAll(ctx context.Context, userID uuid.UUID) (keys []restapikeys.Key, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := o.db.All_RestApiKey_By_UserId(
		ctx,
		dbx.RestApiKey_UserId(userID.Bytes()),
	)
	if err != nil {
		return nil, err
	}

	keys = make([]restapikeys.Key, 0, len(rows))
	for _, row := range rows {
		ID, err := uuid.FromBytes(row.Id)
		if err != nil {
			return nil, err
		}

		userID, err := uuid.FromBytes(row.UserId)
		if err != nil {
			return nil, err
		}

		keys = append(keys, restapikeys.Key{
			ID:        ID,
			UserID:    userID,
			Name:      row.Name,
			Token:     string(row.Token),
			CreatedAt: row.CreatedAt,
			ExpiresAt: row.ExpiresAt,
		})
	}

	return keys, nil
}

// Create creates a new RestAPIKey.
func (o *restApiKeysDB) Create(ctx context.Context, key restapikeys.Key) (_ *restapikeys.Key, err error) {
	defer mon.Task()(&ctx)(&err)

	key.ID, err = uuid.New()
	if err != nil {
		return nil, err
	}

	optional := dbx.RestApiKey_Create_Fields{}
	if key.ExpiresAt == nil {
		optional.ExpiresAt = dbx.RestApiKey_ExpiresAt_Null()
	} else {
		optional.ExpiresAt = dbx.RestApiKey_ExpiresAt(*key.ExpiresAt)
	}
	err = o.db.CreateNoReturn_RestApiKey(ctx, dbx.RestApiKey_Id(key.ID.Bytes()),
		dbx.RestApiKey_UserId(key.UserID.Bytes()), dbx.RestApiKey_Token([]byte(key.Token)),
		dbx.RestApiKey_Name(key.Name), optional)

	return &key, err
}

// Revoke revokes a REST API key by deleting it.
func (o *restApiKeysDB) Revoke(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	deleted, err := o.db.Delete_RestApiKey_By_Id(ctx, dbx.RestApiKey_Id(id.Bytes()))
	if !deleted {
		return sql.ErrNoRows
	}
	return err
}
