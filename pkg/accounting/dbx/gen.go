// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dbx

// go:generate dbx.v1 schema -d postgres -d sqlite3 accounting.dbx .
// go:generate dbx.v1 golang -d postgres -d sqlite3 -p dbx accounting.dbx .
