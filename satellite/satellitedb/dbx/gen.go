// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/cockroachutil"
	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

//go:generate go run ./gen

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
	ShouldRetry = func(driver string, err error) bool {
		if driver == "pgxcockroach" || driver == "cockroach" {
			return cockroachutil.NeedsRetry(err)
		}
		return false
	}
}

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
	var cerr *constraintError
	return errors.As(err, &cerr)
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

// DriverMethods contains both the driver and generated driver methods.
type DriverMethods interface {
	driver
	DialectMethods
	Methods
}

/* Expose internal driver, so we don't have to keep passing them separately to services. */

func (*pgxImpl) AsOfSystemTime(t time.Time) string {
	return dbutil.Postgres.AsOfSystemTime(t)
}

func (*pgxcockroachImpl) AsOfSystemTime(t time.Time) string {
	return dbutil.Cockroach.AsOfSystemTime(t)
}

func (*spannerImpl) AsOfSystemTime(t time.Time) string {
	return dbutil.Spanner.AsOfSystemTime(t)
}

func (*pgxImpl) AsOfSystemInterval(t time.Duration) string {
	return dbutil.Postgres.AsOfSystemInterval(t)
}

func (*pgxcockroachImpl) AsOfSystemInterval(t time.Duration) string {
	return dbutil.Cockroach.AsOfSystemInterval(t)
}

func (*spannerImpl) AsOfSystemInterval(t time.Duration) string {
	return dbutil.Spanner.AsOfSystemInterval(t)
}
