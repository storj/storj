// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

// dbContainerImpl fulfills the migrate.DB interface and the SQLDB interface
type dbContainerImpl struct {
	SQLDB
}

// Schema returns schema
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *dbContainerImpl) Schema() string {
	return ""
}

// Rebind rebind parameters
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *dbContainerImpl) Rebind(s string) string {
	return s
}

// Configure sets the underlining SQLDB connection.
func (db *dbContainerImpl) Configure(sqlDB SQLDB) {
	db.SQLDB = sqlDB
}

// GetDB returns the raw *sql.DB underlying this dbContainerImpl
func (db *dbContainerImpl) GetDB() SQLDB {
	return db.SQLDB
}
