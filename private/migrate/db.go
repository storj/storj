// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"storj.io/storj/private/dbutil/txutil"
)

// DB is the minimal implementation that is needed by migrations.
//
// DB can optionally have `Rebind(string) string` for translating `? queries for the specific database.
type DB interface {
	BeginTx(ctx context.Context, txOptions *sql.TxOptions) (*sql.Tx, error)
	Driver() driver.Driver
}

// DBX contains additional methods for migrations.
type DBX interface {
	DB
	Schema() string
	Rebind(string) string
}

// rebind uses Rebind method when the database has the func.
func rebind(db DB, s string) string {
	if dbx, ok := db.(interface{ Rebind(string) string }); ok {
		return dbx.Rebind(s)
	}
	return s
}

// WithTx runs the given callback in the context of a transaction.
func WithTx(ctx context.Context, db DB, fn func(ctx context.Context, tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	return txutil.ExecuteInTx(ctx, db.Driver(), tx, func() error {
		return fn(ctx, tx)
	})
}
