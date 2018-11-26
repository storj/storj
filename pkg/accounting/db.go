// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/accounting/dbx"
	"storj.io/storj/pkg/utils"
)

// NewDb - constructor for DB
func NewDb(databaseURL string) (*dbx.DB, error) {
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
		db.Close()
		return nil, err
	}
	return db, nil
}
