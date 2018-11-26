// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/accounting/dbx"
)

// NewDb - constructor for DB
func NewDb(driver, source string) (*dbx.DB, error) {
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}
	err = EnsureTables(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// EnsureTables is a method for creating all tables
func EnsureTables(db *dbx.DB) error {
	return migrate.Create("accounting", db)
}
