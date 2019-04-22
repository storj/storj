// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/zeebo/errs"
)

//go:generate dbx.v1 schema -d postgres -d sqlite3 satellitedb.dbx .
//go:generate dbx.v1 golang -d postgres -d sqlite3 satellitedb.dbx .

func init() {
	// do not hide the actual error, necessary for other logic to work
	WrapErr = func(e *Error) error {
		return e.Err
	}
}

// WithTx wraps DB code in a transaction
func (db *DB) WithTx(ctx context.Context, fn func(context.Context, *Tx) error) (err error) {
	tx, err := db.Open(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()
	return fn(ctx, tx)
}
