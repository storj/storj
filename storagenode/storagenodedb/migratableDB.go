// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import "database/sql"

type MigratableDB struct {
	*sql.DB
}

// Schema returns schema
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *MigratableDB) Schema() string {
	return ""
}

// Rebind rebind parameters
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *MigratableDB) Rebind(s string) string {
	return s
}

// Configure sets the underlining *sql.DB connection.
func (db *MigratableDB) Configure(sqlDB *sql.DB) {
	db.DB = sqlDB
}

func (db *MigratableDB) SQLDB() *sql.DB {
	return db.DB
}
