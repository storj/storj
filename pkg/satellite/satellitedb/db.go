// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"

	"context"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

var (
	// Error is the default satellitedb errs class
	Error = errs.Class("satellitedb")
)

// Database contains access to different satellite databases
type Database struct {
	db *dbx.DB
	tx *dbx.Tx

	methods dbx.Methods
}

// New - constructor for DB
func New(driver, source string) (satellite.DB, error) {
	db, err := dbx.Open(driver, source)

	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			driver, source, err)
	}

	database := &Database{
		db:      db,
		methods: db,
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
	return &projects{db.methods}
}

// ProjectMembers is a getter for ProjectMembers repository
func (db *Database) ProjectMembers() satellite.ProjectMembers {
	return &projectMembers{db.methods}
}

// CreateTables is a method for creating all tables for satellitedb
func (db *Database) CreateTables() error {
	return migrate.Create("satellitedb", db.db)
}

// Close is used to close db connection
func (db *Database) Close() error {
	return db.db.Close()
}

// BeginTransaction is a method for opening transaction
func (db *Database) BeginTransaction(ctx context.Context) (err error) {
	if db.db == nil {
		return errs.New("DB is not initialized!")
	}

	db.tx, err = db.db.Open(ctx)
	if err != nil {
		return err
	}

	db.methods = db.tx

	return err
}

// CommitTransaction is a method for committing and closing transaction
func (db *Database) CommitTransaction() error {
	if db.tx == nil {
		return errs.New("begin transaction before commit it!")
	}

	db.methods = db.db

	return db.tx.Commit()
}

// RollbackTransaction is a method for rollback and closing transaction
func (db *Database) RollbackTransaction() error {
	if db.tx == nil {
		return errs.New("begin transaction before rollback it!")
	}

	db.methods = db.db

	return db.tx.Rollback()
}
