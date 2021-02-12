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
	"storj.io/storj/multinode/console"
	"storj.io/storj/multinode/multinodedb/dbx"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
)

var (
	// ensures that multinodeDB implements multinode.DB.
	_ multinode.DB = (*multinodeDB)(nil)

	mon = monkit.Package()

	// Error is the default multinodedb errs class.
	Error = errs.Class("multinodedb internal error")
)

// multinodeDB combines access to different database tables with a record
// of the db driver, db implementation, and db source URL.
// Implementation of multinode.DB interface.
//
// architecture: Master Database
type multinodeDB struct {
	*dbx.DB

	log            *zap.Logger
	driver         string
	implementation dbutil.Implementation
	source         string
}

// Open creates instance of database supports postgres.
func Open(ctx context.Context, log *zap.Logger, databaseURL string) (multinode.DB, error) {
	driver, source, implementation, err := dbutil.SplitConnStr(databaseURL)
	if err != nil {
		return nil, err
	}

	switch implementation {
	case dbutil.SQLite3:
		source = sqlite3SetDefaultOptions(source)
	case dbutil.Postgres:
		source, err = pgutil.CheckApplicationName(source, "multinode")
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

	core := &multinodeDB{
		DB: dbxDB,

		log:            log,
		driver:         driver,
		implementation: implementation,
		source:         source,
	}

	return core, nil
}

// Nodes returns nodes database.
func (db *multinodeDB) Nodes() nodes.DB {
	return &nodesdb{
		methods: db,
	}
}

// Members returns members database.
func (db *multinodeDB) Members() console.Members {
	return &members{
		methods: db,
	}
}

// CreateSchema creates schema.
func (db *multinodeDB) CreateSchema(ctx context.Context) error {
	_, err := db.ExecContext(ctx, db.DB.Schema())
	return err
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
