// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

const (
	// VersionsDBName represents the database name.
	VersionsDBName = "info"

	// VersionsDatabaseFilename represents the database filename.
	VersionsDatabaseFilename = "info.db"
)

// versions represents the database that contains the database schema version history.
type versionsDB struct {
	SQLDB
}

func newVersionsDB() *versionsDB {
	return &versionsDB{}
}

// Configure sets the underlining SQLDB connection.
func (db *versionsDB) Configure(sqlDB SQLDB) {
	db.SQLDB = sqlDB
}

// Rebind rebind parameters
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *versionsDB) Rebind(s string) string { return s }

// Schema returns schema
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *versionsDB) Schema() string { return "" }
