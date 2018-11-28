// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/accounting/dbx"
	"storj.io/storj/pkg/utils"
)

// LastBandwidthTally is a name in the accounting timestamps database
var LastBandwidthTally dbx.Timestamps_Name_Field

func init() {
	LastBandwidthTally = dbx.Timestamps_Name("LastBandwidthTally")
}

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
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
