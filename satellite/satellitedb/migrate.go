// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/internal/migrate"
)

// CreateTables is a method for creating all tables for database
func (db *DB) CreateTables() error {
	switch db.driver {
	case "postgres":
		return db.migratePostgres()
	default:
		return migrate.Create("database", db.db)
	}
}

func (db *DB) migratePostgres() error {
	return migrate.Create("database", db.db)
}
