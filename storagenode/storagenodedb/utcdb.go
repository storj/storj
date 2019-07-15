// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"
)

// utcChecks controls if the time zone checks are enabled.
var utcChecks = false

// utcDB wraps a sql.DB and checks all of the arguments to queries to ensure they are in UTC.
type utcDB struct {
	db *sql.DB
}

// Close closes the database.
func (u utcDB) Close() error { return u.db.Close() }

// Query executes Query after checking all of the arguments.
func (u utcDB) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return u.db.Query(sql, args...)
}

// QueryRow executes QueryRow after checking all of the arguments.
func (u utcDB) QueryRow(sql string, args ...interface{}) *sql.Row {
	// TODO(jeff): figure out a way to return an errored *sql.Row so we can consider
	// enabling all of these checks in production.
	if err := utcCheckArgs(args); err != nil {
		panic(err)
	}
	return u.db.QueryRow(sql, args...)
}

// QueryContext executes QueryContext after checking all of the arguments.
func (u utcDB) QueryContext(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return u.db.QueryContext(ctx, sql, args...)
}

// QueryRowContext executes QueryRowContext after checking all of the arguments.
func (u utcDB) QueryRowContext(ctx context.Context, sql string, args ...interface{}) *sql.Row {
	// TODO(jeff): figure out a way to return an errored *sql.Row so we can consider
	// enabling all of these checks in production.
	if err := utcCheckArgs(args); err != nil {
		panic(err)
	}
	return u.db.QueryRowContext(ctx, sql, args...)
}

// Exec executes Exec after checking all of the arguments.
func (u utcDB) Exec(sql string, args ...interface{}) (sql.Result, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return u.db.Exec(sql, args...)
}

// ExecContext executes ExecContext after checking all of the arguments.
func (u utcDB) ExecContext(ctx context.Context, sql string, args ...interface{}) (sql.Result, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return u.db.ExecContext(ctx, sql, args...)
}

// utcCheckArgs checks the arguments for time.Time values that are not in the UTC location.
func utcCheckArgs(args []interface{}) error {
	if !utcChecks {
		return nil
	}

	for n, arg := range args {
		var t time.Time
		var ok bool

		switch a := arg.(type) {
		case time.Time:
			t, ok = a, true
		case *time.Time:
			if a != nil {
				t, ok = *a, true
			}
		}
		if !ok {
			continue
		}

		if loc := t.Location(); loc != time.UTC {
			return errs.New("invalid timezone on argument %d: %v", n, loc)
		}
	}

	return nil
}
