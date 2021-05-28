// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedb

import (
	"storj.io/storj/private/migrate"
)

// SQLite3Migration returns steps needed for migrating sqlite3 database.
func (db *DB) SQLite3Migration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.migrationDB,
				Description: "Initial setup",
				Version:     0,
				Action: migrate.SQL{
					`CREATE TABLE nodes (
						id BLOB NOT NULL,
						name TEXT NOT NULL,
						public_address TEXT NOT NULL,
						api_secret BLOB NOT NULL,
						PRIMARY KEY ( id )
					); `,
				},
			},
		},
	}
}

// PostgresMigration returns steps needed for migrating postgres database.
func (db *DB) PostgresMigration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.migrationDB,
				Description: "Initial setup",
				Version:     0,
				Action: migrate.SQL{
					`CREATE TABLE nodes (
						id bytea NOT NULL,
						name text NOT NULL,
						public_address text NOT NULL,
						api_secret bytea NOT NULL,
						PRIMARY KEY ( id )
					);`,
				},
			},
		},
	}
}
