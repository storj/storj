// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/zeebo/errs"

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
func New(driver, source string) (*Database, error) {
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
	return &users{db.methods}
}

// Projects is a getter for Projects repository
func (db *Database) Projects() satellite.Projects {
	return &projects{db.methods}
}

// ProjectMembers is a getter for ProjectMembers repository
func (db *Database) ProjectMembers() satellite.ProjectMembers {
	return &projectMembers{db.methods, db.db}
}

// APIKeys is a getter for APIKeys repository
func (db *Database) APIKeys() satellite.APIKeys {
	return &apikeys{db.methods}
}

// CreateTables is a method for creating all tables for satellitedb
func (db *Database) CreateTables() error {
	if db.db == nil {
		return errs.New("Connection is closed")
	}
	return migrate.Create("satellitedb", db.db)
}

// Close is used to close db connection
func (db *Database) Close() error {
	if db.db == nil {
		return errs.New("Connection is closed")
	}
	return db.db.Close()
}

// BeginTx is a method for opening transaction
func (db *Database) BeginTx(ctx context.Context) (satellite.DBTx, error) {
	if db.db == nil {
		return nil, errs.New("DB is not initialized!")
	}

	tx, err := db.db.Open(ctx)
	if err != nil {
		return nil, err
	}

	return &DBTx{
		Database: &Database{
			tx:      tx,
			methods: tx,
		},
	}, nil
}

// DBTx extends Database with transaction scope
type DBTx struct {
	*Database
}

// Commit is a method for committing and closing transaction
func (db *DBTx) Commit() error {
	if db.tx == nil {
		return errs.New("begin transaction before commit it!")
	}

	return db.tx.Commit()
}

// Rollback is a method for rollback and closing transaction
func (db *DBTx) Rollback() error {
	if db.tx == nil {
		return errs.New("begin transaction before rollback it!")
	}

	return db.tx.Rollback()
}
