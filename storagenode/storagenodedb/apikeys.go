// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/storj/storagenode/apikey"
)

// ensures that secretDB implements apikey.DB interface.
var _ apikey.DB = (*secretDB)(nil)

// ErrSecret represents errors from the apikey database.
var ErrSecret = errs.Class("apikey db error")

// SecretDBName represents the database name.
const SecretDBName = "secret"

// secretDB works with node apikey DB.
type secretDB struct {
	dbContainerImpl
}

// Store stores apikey into database.
func (db *secretDB) Store(ctx context.Context, secret apikey.APIKey) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `INSERT INTO secret (
			token,
			created_at
		) VALUES(?,?)`

	_, err = db.ExecContext(ctx, query,
		secret.Secret[:],
		secret.CreatedAt,
	)

	return ErrSecret.Wrap(err)
}

// Check checks if apikey exists in db by token.
func (db *secretDB) Check(ctx context.Context, token apikey.Secret) (err error) {
	defer mon.Task()(&ctx)(&err)

	var bytes []uint8
	var createdAt string

	rowStub := db.QueryRowContext(ctx,
		`SELECT token, created_at FROM secret WHERE token = ?`,
		token[:],
	)

	err = rowStub.Scan(
		&bytes,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apikey.ErrNoSecret.Wrap(err)
		}
		return ErrSecret.Wrap(err)
	}

	return nil
}

// Revoke removes apikey from db.
func (db *secretDB) Revoke(ctx context.Context, secret apikey.Secret) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `DELETE FROM secret WHERE token = ?`

	_, err = db.ExecContext(ctx, query, secret[:])

	return ErrSecret.Wrap(err)
}
