// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accountingdb

import (
	dbx "storj.io/storj/pkg/accounting/accountingdb/dbx"
)

// Database contains access to accounting database
type Database struct { 
	db *dbx.DB
}

// New - constructor for DB
func New(driver, source string) (*Database, error) {
	db, err := dbx.Open(driver, source)

	if err != nil {
		return &Database{}, err
	}

	database := &Database{
		db: db,
	}

	return database, nil
}
