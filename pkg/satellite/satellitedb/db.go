// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/pkg/satellite"

	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

// Database contains access to different satellite databases
type Database struct {
	db *dbx.DB
}

// New - constructor for DB
func New(driver, source string) (satellite.DB, error) {
	db, err := dbx.Open(driver, source)

	if err != nil {
		return nil, err
	}

	database := &Database{
		db: db,
	}

	return database, nil
}

// Users is getter for Users repository
func (db *Database) Users() satellite.Users {
	return &users{db.db}
}

// Companies is getter for Companies repository
func (db *Database) Companies() satellite.Companies {
	return &companies{db.db}
}

// CreateTables is a method for creating all tables for satellitedb
func (db *Database) CreateTables() error {
	_, err := db.db.Exec(db.db.Schema())

	return err
}

// Close is used to close db connection
func (db *Database) Close() error {
	return db.db.Close()
}
