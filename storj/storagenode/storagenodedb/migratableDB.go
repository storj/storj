// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"database/sql"
)

// migratableDB fulfills the migrate.DB interface and the SQLDB interface
type migratableDB struct {
	*sql.DB
}

// Schema returns schema
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *migratableDB) Schema() string {
	return ""
}

// Rebind rebind parameters
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *migratableDB) Rebind(s string) string {
	return s
}

// Configure sets the underlining SQLDB connection.
func (db *migratableDB) Configure(sqlDB *sql.DB) {
	db.DB = sqlDB
}

// GetDB returns the raw *sql.DB underlying this migratableDB
func (db *migratableDB) GetDB() *sql.DB {
	return db.DB
}
