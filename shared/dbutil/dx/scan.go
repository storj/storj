// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dx

import (
	"database/sql"
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
