// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"encoding/hex"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// ensures that apiKeyTails implements console.APIKeyTails.
var _ console.APIKeyTails = (*apiKeyTails)(nil)

type apiKeyTails struct {
	db        tagsql.DB
	dbMethods dbx.DriverMethods
	impl      dbutil.Implementation
}

// Upsert is a method for inserting or updating console.APIKeyTail in the database.
func (tails *apiKeyTails) Upsert(ctx context.Context, tail *console.APIKeyTail) (_ *console.APIKeyTail, err error) {
	defer mon.Task()(&ctx)(&err)

	if tail == nil {
		return nil, Error.New("tail is nil")
	}

	dbxTail, err := tails.dbMethods.Replace_ApiKeyTail(
		ctx,
		dbx.ApiKeyTail_Tail(tail.Tail),
		dbx.ApiKeyTail_ParentTail(tail.ParentTail),
		dbx.ApiKeyTail_Caveat(tail.Caveat),
		dbx.ApiKeyTail_LastUsed(tail.LastUsed),
		dbx.ApiKeyTail_Create_Fields{RootKeyId: dbx.ApiKeyTail_RootKeyId(tail.RootKeyID.Bytes())},
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return fromDBXAPIKeyTail(dbxTail)
}

// UpsertBatch is a method for inserting or updating a batch of console.APIKeyTails in the database.
func (tails *apiKeyTails) UpsertBatch(ctx context.Context, batch []console.APIKeyTail) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(batch) == 0 {
		return nil
	}

	switch tails.impl {
	case dbutil.Postgres:
		query := tails.dbMethods.Rebind(`
			INSERT INTO api_key_tails
				(root_key_id, tail, parent_tail, caveat, last_used)
			SELECT
				unnest(?::BYTEA[]),
				unnest(?::BYTEA[]),
				unnest(?::BYTEA[]),
				unnest(?::BYTEA[]),
				unnest(?::timestamptz[])
			ON CONFLICT (tail) DO UPDATE
				SET last_used = EXCLUDED.last_used
        `)
		_, err = tails.dbMethods.ExecContext(ctx, query, convertUpsertBatchArgs(batch)...)
	case dbutil.Cockroach:

		query := tails.dbMethods.Rebind(`
			UPSERT INTO api_key_tails
				(root_key_id, tail, parent_tail, caveat, last_used)
			SELECT * FROM UNNEST(
				?::BYTEA[],
				?::BYTEA[],
				?::BYTEA[],
				?::BYTEA[],
				?::timestamptz[]
			)
        `)
		_, err = tails.dbMethods.ExecContext(ctx, query, convertUpsertBatchArgs(batch)...)
	case dbutil.Spanner:
		muts := make([]*spanner.Mutation, 0, len(batch))
		for _, it := range batch {
			muts = append(muts, spanner.InsertOrUpdate(
				"api_key_tails",
				[]string{"root_key_id", "tail", "parent_tail", "caveat", "last_used"},
				[]any{
					it.RootKeyID[:],
					it.Tail,
					it.ParentTail,
					it.Caveat,
					it.LastUsed,
				},
			))
		}

		err = spannerutil.UnderlyingClient(ctx, tails.db, func(client *spanner.Client) error {
			_, err := client.Apply(ctx, muts, spanner.TransactionTag("upsert-batch-api-key-tails"))
			return err
		})
	default:
		err = errs.New("unsupported database dialect: %s", tails.impl)
	}
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// GetByTail retrieves console.APIKeyTail for given key tail.
func (tails *apiKeyTails) GetByTail(ctx context.Context, tail []byte) (_ *console.APIKeyTail, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTail, err := tails.dbMethods.Get_ApiKeyTail_By_Tail(ctx, dbx.ApiKeyTail_Tail(tail))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return fromDBXAPIKeyTail(dbxTail)
}

// CheckExistenceBatch checks the existence of multiple tails in a single query.
func (tails *apiKeyTails) CheckExistenceBatch(ctx context.Context, tailsToCheck [][]byte) (_ map[string]bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(tailsToCheck) == 0 {
		return nil, nil
	}

	result := make(map[string]bool, len(tailsToCheck))
	for _, tail := range tailsToCheck {
		result[hex.EncodeToString(tail)] = false
	}

	var rows tagsql.Rows
	switch tails.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		query := tails.dbMethods.Rebind(`SELECT tail FROM api_key_tails WHERE tail = ANY(?)`)
		rows, err = tails.dbMethods.QueryContext(ctx, query, pgutil.ByteaArray(tailsToCheck))
	case dbutil.Spanner:
		query := `SELECT tail FROM api_key_tails WHERE tail IN UNNEST(?)`
		rows, err = tails.dbMethods.QueryContext(ctx, query, tailsToCheck)
	default:
		return nil, Error.New("unsupported database implementation")
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var foundTail []byte
		if err = rows.Scan(&foundTail); err != nil {
			return nil, Error.Wrap(err)
		}
		result[hex.EncodeToString(foundTail)] = true
	}
	if err = rows.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// fromDBXAPIKeyTail converts *dbx.ApiKeyTail to *console.APIKeyTail.
func fromDBXAPIKeyTail(dbxTail *dbx.ApiKeyTail) (*console.APIKeyTail, error) {
	if dbxTail == nil {
		return nil, Error.New("dbx tail is nil")
	}

	rootKeyID, err := uuid.FromBytes(dbxTail.RootKeyId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &console.APIKeyTail{
		RootKeyID:  rootKeyID,
		Tail:       dbxTail.Tail,
		ParentTail: dbxTail.ParentTail,
		Caveat:     dbxTail.Caveat,
		LastUsed:   dbxTail.LastUsed,
	}, nil
}

func convertUpsertBatchArgs(batch []console.APIKeyTail) []any {
	rootKeyIDs := make([]uuid.UUID, 0, len(batch))
	tailsArr := make([][]byte, 0, len(batch))
	parents := make([][]byte, 0, len(batch))
	caveats := make([][]byte, 0, len(batch))
	lastUsedArr := make([]time.Time, 0, len(batch))

	for _, it := range batch {
		rootKeyIDs = append(rootKeyIDs, it.RootKeyID)
		tailsArr = append(tailsArr, it.Tail)
		parents = append(parents, it.ParentTail)
		caveats = append(caveats, it.Caveat)
		lastUsedArr = append(lastUsedArr, it.LastUsed)
	}

	return []any{
		pgutil.UUIDArray(rootKeyIDs),
		pgutil.ByteaArray(tailsArr),
		pgutil.ByteaArray(parents),
		pgutil.ByteaArray(caveats),
		pgutil.TimestampTZArray(lastUsedArr),
	}
}
