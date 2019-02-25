// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"
)

//go:generate dbx.v1 schema -d postgres -d sqlite3 satellitedb.dbx .
//go:generate dbx.v1 golang -d postgres -d sqlite3 satellitedb.dbx .

func init() {
	// catch dbx errors
	c := errs.Class("satellitedb")
	WrapErr = func(e *Error) error {
		if e.Code == ErrorCode_NoRows {
			return e.Err
		}
		return c.Wrap(e)
	}
}
