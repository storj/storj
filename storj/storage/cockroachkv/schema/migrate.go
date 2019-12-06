// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package schema

import (
	"database/sql"

	"github.com/zeebo/errs"
)

// PrepareDB creates the pathdata tables if they don't already exist.
func PrepareDB(db *sql.DB) (err error) {
	// Note: the buckets table is unused. It exists here to ease importing
	// backups from postgres. Similarly, the bucket column in pathdata is
	// also unused and exists to ease imports.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS buckets (
			bucketname BYTES PRIMARY KEY,
			delim INT8 NOT NULL
		);
		CREATE TABLE IF NOT EXISTS pathdata (
			fullpath BYTEA PRIMARY KEY,
			metadata BYTEA NOT NULL,
			bucket BYTEA
		);
	`)
	return errs.Wrap(err)
}
