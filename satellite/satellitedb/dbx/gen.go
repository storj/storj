// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbx

import (
	"context"
	"fmt"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	// load our cockroach sql driver for anywhere that uses this dbx.Open.
	_ "storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/dbutil/txutil"
	"storj.io/storj/private/tagsql"
)

//go:generate sh gen.sh

var mon = monkit.Package()

func init() {
	// catch dbx errors
	class := errs.Class("satellitedb")
	WrapErr = func(e *Error) error {
		switch e.Code {
		case ErrorCode_NoRows:
			return e.Err
		case ErrorCode_ConstraintViolation:
			return class.Wrap(&constraintError{e.Constraint, e.Err})
		}
		return class.Wrap(e)
	}
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error { return e.Err }

// Cause returns the underlying error.
func (e *Error) Cause() error { return e.Err }

type constraintError struct {
	constraint string
	err        error
}

// Unwrap returns the underlying error.
func (err *constraintError) Unwrap() error { return err.err }

// Cause returns the underlying error.
func (err *constraintError) Cause() error { return err.err }

// IsConstraintError returns true if the error is a constraint error.
func IsConstraintError(err error) bool {
	_, ok := err.(*constraintError)
	return ok
}

// Error implements the error interface.
func (err *constraintError) Error() string {
	return fmt.Sprintf("violates constraint %q: %v", err.constraint, err.err)
}

// WithTx wraps DB code in a transaction.
func (db *DB) WithTx(ctx context.Context, fn func(context.Context, *Tx) error) (err error) {
	return txutil.WithTx(ctx, db, nil, func(ctx context.Context, tx tagsql.Tx) error {
		return fn(ctx, &Tx{
			Tx:        tx,
			txMethods: db.wrapTx(tx),
		})
	})
}
