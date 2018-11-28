// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accountingdb

import (
	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/accounting/accountingdb/dbx"
	"storj.io/storj/pkg/utils"
)

// Database contains access to accounting database
type Database struct { 
	db *dbx.DB
}

// New - constructor for DB
func New(databaseURL string) (*Database, error) {
	dbURL, err := utils.ParseURL(databaseURL)
	if err != nil {
		return nil, err
	}
	db, err := dbx.Open(dbURL.Scheme, dbURL.Path)
	if err != nil {
		return nil, err
	}
	err = migrate.Create("accounting", db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Database{db: db}, nil
}

func (D *Database) Close() error {
	return D.db.Close()
}