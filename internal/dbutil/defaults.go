// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"database/sql"
	"flag"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	maxIdleConns    = flag.Int("db.max_idle_conns", 50, "default value for database connections, -1 means the stdlib default")
	maxOpenConns    = flag.Int("db.max_open_conns", 100, "default value for database connections, -1 means the stdlib default")
	connMaxLifetime = flag.Duration("db.conn_max_lifetime", -1, "default value for database connections, -1 means the stdlib default")
)

func Configure(db *sql.DB, mon *monkit.Scope) {
	if *maxIdleConns >= 0 {
		db.SetMaxIdleConns(*maxIdleConns)
	}
	if *maxOpenConns >= 0 {
		db.SetMaxOpenConns(*maxOpenConns)
	}
	if *connMaxLifetime >= 0 {
		db.SetConnMaxLifetime(*connMaxLifetime)
	}
	mon.Chain("db_stats", monkit.StatSourceFunc(
		func(cb func(name string, val float64)) {
			monkit.StatSourceFromStruct(db.Stats()).Stats(cb)
		}))
}
