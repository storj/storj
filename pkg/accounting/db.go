// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/accounting/dbx"
	"storj.io/storj/pkg/utils"
)

var (
	// Error is the default accountingdb errs class
	Error = errs.Class("accountingdb")

	// LastBandwidthTally is a name in the accounting timestamps database
	LastBandwidthTally = dbx.Timestamps_Name("LastBandwidthTally")
)

// Database contains access to accounting database
type Database struct {
	db *dbx.DB
}

// NewDB - constructor for Database
func NewDB(databaseURL string) (*Database, error) {
	dbURL, err := utils.ParseURL(databaseURL)
	if err != nil {
		return nil, err
	}
	db, err := dbx.Open(dbURL.Scheme, dbURL.Path)
	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			dbURL.Scheme, dbURL.Path, err)
	}
	err = migrate.Create("accounting", db)
	if err != nil {
		return nil, utils.CombineErrors(err, db.Close())
	}
	return &Database{db: db}, nil
}

// BeginTx is used to open db connection
func (db *Database) BeginTx(ctx context.Context) (*dbx.Tx, error) {
	return db.db.Open(ctx)
}

// Close is used to close db connection
func (db *Database) Close() error {
	return db.db.Close()
}

// FindLastBwTally returns the timestamp of the last bandwidth tally
func (db *Database) FindLastBwTally(ctx context.Context) (*dbx.Value_Row, error) {
	return db.db.Find_Timestamps_Value_By_Name(ctx, LastBandwidthTally)
}
