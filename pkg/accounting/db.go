// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
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

// NewDb - constructor for DB
func NewDb(databaseURL string) (*dbx.DB, error) {
	driver, source, err := utils.SplitDBURL(databaseURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, Error.New("failed opening database %q, %q: %v",
			driver, source, err)
	}
	err = migrate.Create("accounting", db)
	if err != nil {
		_ = db.Close()
		return nil, Error.Wrap(err)
	}
	return db, nil
}
