// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledbx

//go:generate dbx.v1 schema -d postgres -d sqlite3 consoledb.dbx .
//go:generate dbx.v1 golang -d postgres -d sqlite3 consoledb.dbx .
