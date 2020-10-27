// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/storagenode/secret"
)

// ensures that secretDB implements secret.Secret interface.
var _ secret.DB = (*secretDB)(nil)

// ErrSecret represents errors from the secret database.
var ErrSecret = errs.Class("secret db error")

// SecretDBName represents the database name.
const SecretDBName = "secret"

// secretDB works with node secret DB.
type secretDB struct {
	dbContainerImpl
}

// Store stores now storagenode's uniq secret to database.
func (db *secretDB) Store(ctx context.Context, secret secret.UniqSecret) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `INSERT INTO secret (
			token,
			created_at
		) VALUES(?,?)`

	_, err = db.ExecContext(ctx, query,
		secret.Secret,
		secret.CreatedAt,
	)

	return ErrSecret.Wrap(err)
}

// Check checks if uniq secret exists in db by token.
func (db *secretDB) Check(ctx context.Context, token uuid.UUID) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	result := secret.UniqSecret{}

	rowStub := db.QueryRowContext(ctx,
		`SELECT token, created_at FROM secret WHERE token = ?`,
		token,
	)

	err = rowStub.Scan(
		&result.Secret,
		&result.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, secret.ErrNoSecret.Wrap(err)
		}
		return false, ErrSecret.Wrap(err)
	}

	return true, nil
}

// Revoke removes token from db.
func (db *secretDB) Revoke(ctx context.Context, token uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `DELETE FROM secret WHERE token = ?`

	_, err = db.ExecContext(ctx, query, token)

	return ErrSecret.Wrap(err)
}
