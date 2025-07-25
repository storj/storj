// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that apiKeyTails implements console.APIKeyTails.
var _ console.APIKeyTails = (*apiKeyTails)(nil)

type apiKeyTails struct {
	db dbx.DriverMethods
}

// Upsert is a method for inserting or updating console.APIKeyTail in the database.
func (tails *apiKeyTails) Upsert(ctx context.Context, tail *console.APIKeyTail) (_ *console.APIKeyTail, err error) {
	defer mon.Task()(&ctx)(&err)

	if tail == nil {
		return nil, Error.New("tail is nil")
	}

	dbxTail, err := tails.db.Replace_ApiKeyTail(
		ctx,
		dbx.ApiKeyTail_Tail(tail.Tail),
		dbx.ApiKeyTail_ParentTail(tail.ParentTail),
		dbx.ApiKeyTail_Caveat(tail.Caveat),
		dbx.ApiKeyTail_LastUsed(tail.LastUsed),
		dbx.ApiKeyTail_Create_Fields{RootKeyId: dbx.ApiKeyTail_RootKeyId(tail.RootKeyID.Bytes())},
	)
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKeyTail(dbxTail)
}

// GetByTail retrieves console.APIKeyTail for given key tail.
func (tails *apiKeyTails) GetByTail(ctx context.Context, tail []byte) (_ *console.APIKeyTail, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTail, err := tails.db.Get_ApiKeyTail_By_Tail(ctx, dbx.ApiKeyTail_Tail(tail))
	if err != nil {
		return nil, err
	}

	return fromDBXAPIKeyTail(dbxTail)
}

// fromDBXAPIKeyTail converts *dbx.ApiKeyTail to *console.APIKeyTail.
func fromDBXAPIKeyTail(dbxTail *dbx.ApiKeyTail) (*console.APIKeyTail, error) {
	if dbxTail == nil {
		return nil, Error.New("dbx tail is nil")
	}

	rootKeyID, err := uuid.FromBytes(dbxTail.RootKeyId)
	if err != nil {
		return nil, err
	}

	return &console.APIKeyTail{
		RootKeyID:  rootKeyID,
		Tail:       dbxTail.Tail,
		ParentTail: dbxTail.ParentTail,
		Caveat:     dbxTail.Caveat,
		LastUsed:   dbxTail.LastUsed,
	}, nil
}
