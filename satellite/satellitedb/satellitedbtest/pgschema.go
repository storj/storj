// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/dbutil/pgutil/pgtest"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// NewPostgres returns the default postgres satellite.DB for testing.
func NewPostgres(log *zap.Logger, schema string) (satellite.DB, error) {
	if err := PostgresDefined(); err != nil {
		return nil, err
	}

	db, err := satellitedb.New(log, pgutil.ConnstrWithSchema(*pgtest.ConnStr, schema))
	if err != nil {
		return nil, err
	}

	return &SchemaDB{
		DB:       db,
		Schema:   schema,
		AutoDrop: true,
	}, nil
}

// PostgresDefined returns an error when the --postgres-test-db or STORJ_POSTGRES_TEST is not set for tests.
func PostgresDefined() error {
	if *pgtest.ConnStr == "" {
		return errs.New("flag --postgres-test-db or environment variable STORJ_POSTGRES_TEST not defined for PostgreSQL test database")
	}
	return nil
}

// SchemaDB implements automatic schema handling for satellite.DB
type SchemaDB struct {
	satellite.DB

	Schema   string
	AutoDrop bool
}

// TestDBAccess for raw database access,
// should not be used outside of migration tests.
func (db *SchemaDB) TestDBAccess() *dbx.DB {
	return db.DB.(interface{ TestDBAccess() *dbx.DB }).TestDBAccess()
}

// CreateTables creates the schema and creates tables.
func (db *SchemaDB) CreateTables() error {
	err := db.DB.CreateSchema(db.Schema)
	if err != nil {
		return err
	}

	return db.DB.CreateTables()
}

// Close closes the database and drops the schema, when `AutoDrop` is set.
func (db *SchemaDB) Close() error {
	var dropErr error
	if db.AutoDrop {
		dropErr = db.DB.DropSchema(db.Schema)
	}

	return errs.Combine(dropErr, db.DB.Close())
}
