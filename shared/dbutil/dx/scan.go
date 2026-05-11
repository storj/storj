// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dx

import (
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
)

// ScanRow returns a Query.Do that scans exactly one row into dest. It returns
// sql.ErrNoRows if the result set is empty. Additional rows beyond the first
// are ignored.
func ScanRow(dest ...any) func(Rows) error {
	return func(rows Rows) error {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return sql.ErrNoRows
		}
		return rows.Scan(dest...)
	}
}

// ScanRowOptional returns a Query.Do that scans at most one row into dest. If
// the result set is empty, dest is left untouched and a nil error is returned.
// Additional rows beyond the first are ignored.
func ScanRowOptional(dest ...any) func(Rows) error {
	return func(rows Rows) error {
		if !rows.Next() {
			return rows.Err()
		}
		return rows.Scan(dest...)
	}
}

// ScanFirstRow wraps the (rows, err) result of a QueryContext call so a
// caller can scan exactly one row out of a multi-statement query without
// repeating the result-set / drain / close plumbing.
//
// The returned function walks past any leading empty result sets, scans
// one row into dest, drains any trailing result sets, and closes rows.
// It is intended for the INSERT + SELECT pattern (and similar) where the
// SELECT is the only result-producing statement and preceding statements
// may surface as empty result sets on some drivers.
//
// If err is non-nil, the returned function returns it without touching
// rows. If no result set yields a row, it returns sql.ErrNoRows.
//
// Usage:
//
//	err := dx.ScanFirstRow(db.QueryContext(ctx, "INSERT ...; SELECT ..."))(&dest)
func ScanFirstRow(rows tagsql.Rows, err error) func(dest ...any) error {
	if err != nil {
		return func(...any) error { return err }
	}
	return func(dest ...any) (rerr error) {
		defer func() { rerr = errs.Combine(rerr, rows.Err(), rows.Close()) }()
		for {
			if rows.Next() {
				break
			}
			if !rows.NextResultSet() {
				if e := rows.Err(); e != nil {
					return e
				}
				return sql.ErrNoRows
			}
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		for rows.NextResultSet() {
		}
		return nil
	}
}
