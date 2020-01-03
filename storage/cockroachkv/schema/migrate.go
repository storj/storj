// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package schema

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
)

// PrepareDB creates the pathdata tables if they don't already exist.
func PrepareDB(db *sql.DB) (err error) {
	var dbName string
	if err := db.QueryRow(`SELECT current_database();`).Scan(&dbName); err != nil {
		return errs.Wrap(err)
	}

	// Note: the buckets table is unused. It exists here to ease importing
	// backups from postgres. Similarly, the bucket column in pathdata is
	// also unused and exists to ease imports.
	_, err = db.Exec(fmt.Sprintf(`
		CREATE DATABASE IF NOT EXISTS %s;
		CREATE TABLE IF NOT EXISTS buckets (
			bucketname BYTES PRIMARY KEY,
			delim INT8 NOT NULL
		);
		CREATE TABLE IF NOT EXISTS pathdata (
			fullpath BYTEA PRIMARY KEY,
			metadata BYTEA NOT NULL,
			bucket BYTEA
		);
	`, pq.QuoteIdentifier(dbName)))
	return errs.Wrap(err)
}
