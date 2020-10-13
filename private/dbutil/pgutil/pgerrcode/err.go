// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package pgerrcode implements postgres error extraction without depending on a postgres
// library.
package pgerrcode

import "errors"

// FromError returns the 5-character PostgreSQL error code string associated
// with the given error, if any.
func FromError(err error) string {
	var sqlStateErr errWithSQLState
	if errors.As(err, &sqlStateErr) {
		return sqlStateErr.SQLState()
	}
	return ""
}

// errWithSQLState is an interface supported by error classes corresponding
// to PostgreSQL errors from certain drivers. This is satisfied, in particular,
// by pgx (*pgconn.PgError) and may be adopted by other types. An effort is
// apparently underway to get lib/pq to add this interface.
type errWithSQLState interface {
	SQLState() string
}
