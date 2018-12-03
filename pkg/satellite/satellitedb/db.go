// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/internal/migrate"
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

// Users is getter a for Users repository
func (db *Database) Users() satellite.Users {
	return &users{db.db}
}

// Companies is a getter for Companies repository
func (db *Database) Companies() satellite.Companies {
	return &companies{db.db}
}

// Projects is a getter for Projects repository
func (db *Database) Projects() satellite.Projects {
	return &projects{db.db}
}

// ProjectMembers is a getter for ProjectMembers repository
func (db *Database) ProjectMembers() satellite.ProjectMembers {
	return &projectMembers{db.db}
}

// CreateTables is a method for creating all tables for satellitedb
func (db *Database) CreateTables() error {
	return migrate.Create("satellitedb", db.db)
}

// Close is used to close db connection
func (db *Database) Close() error {
	return db.db.Close()
}
