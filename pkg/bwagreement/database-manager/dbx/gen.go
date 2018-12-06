// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

//go:generate dbx.v1 golang -d postgres -d sqlite3  bwagreement.dbx .
//go:generate dbx.v1 schema -d postgres -d sqlite3  bwagreement.dbx .

import (
	"github.com/zeebo/errs"
)

func init() {
	// catch dbx errors
	c := errs.Class("bwagreementdb")
	WrapErr = func(e *Error) error { return c.Wrap(e) }
}
