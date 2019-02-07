// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

// +build ignore

import (
	"database/sql"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"storj.io/storj/internal/migrate"
)

type DB struct {
	log *zap.Logger
	dir string
	db  *sql.DB
}

func (db *DB) Migrate() error {
	m := migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				Description: "Initial Table",
				Version:     1,
				Action: migrate.SQL{
					`CREATE TABLE users ()`,
				},
			},
			{
				Description: "Move files",
				Version:     2,
				Action:      migrate.Func(db.migrateFiles),
			},
		},
	}

	return m.Run(db.log.Named("migrate"), db.db)
}

func (db *DB) migrateFiles(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
	return os.Rename(filepath.Join(db.dir, "alpha.txt"), filepath.Join(db.dir, "beta.txt"))

}
