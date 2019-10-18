// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

import (
	"github.com/zeebo/errs"

	"storj.io/storj/satellite"
)

// SchemaDB implements automatic schema handling for satellite.DB
type SchemaDB struct {
	satellite.DB

	Schema   string
	AutoDrop bool
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

	closeErr := db.DB.Close()
	return errs.Combine(closeErr, dropErr)
}
