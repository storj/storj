// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

const (
	// LegacyInfoDBName represents the database name.
	LegacyInfoDBName = "info"
)

// legacyInfoDB represents the database that contains the original legacy sqlite3 database.
type legacyInfoDB struct {
	SQLDB
}

func newLegacyInfoDB() *legacyInfoDB {
	return &legacyInfoDB{}
}

// Configure sets the underlining SQLDB connection.
func (db *legacyInfoDB) Configure(sqlDB SQLDB) {
	db.SQLDB = sqlDB
}

// Rebind rebind parameters
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *legacyInfoDB) Rebind(s string) string { return s }

// Schema returns schema
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *legacyInfoDB) Schema() string { return "" }
