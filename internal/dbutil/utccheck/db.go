// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utccheck

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"
)

// DB wraps a sql.DB and checks all of the arguments to queries to ensure they are in UTC.
type DB struct {
	db *sql.DB
}

// New creates a new database that checks that all time arguments are UTC.
func New(db *sql.DB) *DB {
	return &DB{db: db}
}

// Close closes the database.
func (db DB) Close() error { return db.db.Close() }

// Query executes Query after checking all of the arguments.
func (db DB) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return db.db.Query(sql, args...)
}

// QueryRow executes QueryRow after checking all of the arguments.
func (db DB) QueryRow(sql string, args ...interface{}) *sql.Row {
	// TODO(jeff): figure out a way to return an errored *sql.Row so we can consider
	// enabling all of these checks in production.
	if err := utcCheckArgs(args); err != nil {
		panic(err)
	}
	return db.db.QueryRow(sql, args...)
}

// QueryContext executes QueryContext after checking all of the arguments.
func (db DB) QueryContext(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return db.db.QueryContext(ctx, sql, args...)
}

// QueryRowContext executes QueryRowContext after checking all of the arguments.
func (db DB) QueryRowContext(ctx context.Context, sql string, args ...interface{}) *sql.Row {
	// TODO(jeff): figure out a way to return an errored *sql.Row so we can consider
	// enabling all of these checks in production.
	if err := utcCheckArgs(args); err != nil {
		panic(err)
	}
	return db.db.QueryRowContext(ctx, sql, args...)
}

// Exec executes Exec after checking all of the arguments.
func (db DB) Exec(sql string, args ...interface{}) (sql.Result, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return db.db.Exec(sql, args...)
}

// ExecContext executes ExecContext after checking all of the arguments.
func (db DB) ExecContext(ctx context.Context, sql string, args ...interface{}) (sql.Result, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}
	return db.db.ExecContext(ctx, sql, args...)
}

// utcCheckArgs checks the arguments for time.Time values that are not in the UTC location.
func utcCheckArgs(args []interface{}) error {
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
