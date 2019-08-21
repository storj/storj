// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

// versions represents the database that contains the database schema version history.
type versionsDB struct {
	location string
	SQLDB
}

func newVersionsDB(db SQLDB, location string) *versionsDB {
	return &versionsDB{
		location: location,
		SQLDB:    db,
	}
}

// Rebind rebind parameters
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *versionsDB) Rebind(s string) string { return s }

// Schema returns schema
// These are implemented because the migrate.DB interface requires them.
// Maybe in the future we should untangle those.
func (db *versionsDB) Schema() string { return "" }
