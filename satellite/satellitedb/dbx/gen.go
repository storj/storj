// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

//go:generate dbx.v1 golang -d postgres -d sqlite3 satellitedb.dbx .
//go:generate dbx.v1 schema -d postgres -d sqlite3 satellitedb.dbx .
