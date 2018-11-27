// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dbx

//go:generate dbx.v1 golang -d sqlite3 -p dbx satellitedb.dbx .
//go:generate dbx.v1 schema -d sqlite3 satellitedb.dbx .
