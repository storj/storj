// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"database/sql"
)

// DB is the minimal implementation that is needed by migrations.
//
// DB can optionally have `Rebind(string) string` for translating `? queries for the specific database.
type DB interface {
	Begin() (*sql.Tx, error)
}

// DBX contains additional methods for migrations.
type DBX interface {
	DB
	Schema() string
	Rebind(string) string
}

// rebind uses Rebind method when the database has the func.
func rebind(db DB, s string) string {
	if dbx, ok := db.(interface{ Rebind(string) string }); ok {
		return dbx.Rebind(s)
	}
	return s
}
