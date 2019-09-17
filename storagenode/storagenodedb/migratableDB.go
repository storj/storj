// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

type migratableDB struct {
	SQLDB
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
func (db *migratableDB) Configure(sqlDB SQLDB) {
	db.SQLDB = sqlDB
}
