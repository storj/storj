// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedb

import (
	"context"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/multinode"
	"storj.io/storj/multinode/multinodedb/dbx"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

var (
	// ensures that multinodeDB implements multinode.DB.
	_ multinode.DB = (*DB)(nil)

	mon = monkit.Package()

	// Error is the default multinodedb errs class.
	Error = errs.Class("multinodedb")
)

// DB combines access to different database tables with a record
// of the db driver, db implementation, and db source URL.
// Implementation of multinode.DB interface.
//
// architecture: Master Database
type DB struct {
	*dbx.DB

	log            *zap.Logger
	driver         string
	source         string
	implementation dbutil.Implementation
	migrationDB    tagsql.DB
}

// Open creates instance of database supports postgres.
func Open(ctx context.Context, log *zap.Logger, databaseURL string) (*DB, error) {
	driver, source, implementation, err := dbutil.SplitConnStr(databaseURL)
	if err != nil {
		return nil, err
	}

	switch implementation {
	case dbutil.SQLite3:
		source = sqlite3SetDefaultOptions(source)
	case dbutil.Postgres:
		source, err = pgutil.EnsureApplicationName(source, "multinode")
		if err != nil {
			return nil, err
		}
	default:
		return nil, Error.New("unsupported driver %q", driver)
	}

	dbxDB, err := dbx.Open(driver, source)
	if err != nil {
		return nil, Error.New("failed opening database via DBX at %q: %v",
			source, err)
	}
	log.Debug("Connected to:", zap.String("db source", source))

	dbutil.Configure(ctx, dbxDB.DB, "multinodedb", mon)

	core := &DB{
		DB: dbxDB,

		log:            log,
		driver:         driver,
		implementation: implementation,
		source:         source,
	}

	core.migrationDB = core

	return core, nil
}

// Nodes returns nodes database.
func (db *DB) Nodes() nodes.DB {
	return &nodesdb{
		methods: db,
	}
}

// MigrateToLatest migrates db to the latest version.
func (db DB) MigrateToLatest(ctx context.Context) error {
	var migration *migrate.Migration

	switch db.implementation {
	case dbutil.SQLite3:
		migration = db.SQLite3Migration()
	case dbutil.Postgres:
		migration = db.PostgresMigration()
	default:
		return migrate.Create(ctx, "database", db.DB)
	}
	return migration.Run(ctx, db.log)
}

// sqlite3SetDefaultOptions sets default options for disk-based db with URI filename source string
// if no options were set.
func sqlite3SetDefaultOptions(source string) string {
	if !strings.HasPrefix(source, "file:") {
		return source
	}
	// do not set anything for in-memory db
	if strings.HasPrefix(source, "file::memory:") {
		return source
	}
	if strings.Contains(source, "?") {
		return source
	}

	return source + "?_journal=WAL&_busy_timeout=10000"
}
