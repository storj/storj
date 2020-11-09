// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedb

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/multinode"
	"storj.io/storj/multinode/console"
	"storj.io/storj/multinode/multinodedb/dbx"
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
	// TODO: do we need cockroach implementation?
	if implementation != dbutil.Postgres && implementation != dbutil.Cockroach {
		return nil, Error.New("unsupported driver %q", driver)
	}

	source = pgutil.CheckApplicationName(source)

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
func (db *multinodeDB) Nodes() console.Nodes {
	return &nodes{
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
