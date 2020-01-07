// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package txutil provides safe transaction-encapsulation functions which have retry
// semantics as necessary.
package txutil

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/cockroachdb/cockroach-go/crdb"
	"github.com/zeebo/errs"
	"storj.io/storj/private/dbutil/cockroachutil"
)

// txLike is the minimal interface for transaction-like objects to work with the necessary retry
// semantics on things like CockroachDB.
type txLike interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	Commit() error
	Rollback() error
}

// ExecuteInTx runs the fn callback inside the specified transaction, restarting the transaction
// as necessary (for systems like CockroachDB), and committing or rolling back the transaction
// depending on whether fn returns an error.
//
// In most cases, WithTx() is to be preferred, but this variant is useful when working with a db
// that isn't an *sql.DB.
func ExecuteInTx(ctx context.Context, dbDriver driver.Driver, tx txLike, fn func() error) (err error) {
	if _, ok := dbDriver.(*cockroachutil.Driver); ok {
		return crdb.ExecuteInTx(ctx, tx, fn)
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()
	return fn()
}

// WithTx starts a transaction on the given sql.DB. The transaction is started in the appropriate
// manner, and will be restarted if appropriate. While in the transaction, fn is called with a
// handle to the transaction in order to make use of it. If fn returns an error, the transaction
// is rolled back. If fn returns nil, the transaction is committed.
//
// If fn has any side effects outside of changes to the database, they must be idempotent! fn may
// be called more than one time.
func WithTx(ctx context.Context, db *sql.DB, txOpts *sql.TxOptions, fn func(context.Context, *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, txOpts)
	if err != nil {
		return err
	}
	return ExecuteInTx(ctx, db.Driver(), tx, func() error {
		return fn(ctx, tx)
	})
}
