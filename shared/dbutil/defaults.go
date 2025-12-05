// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"context"
	"database/sql"
	"flag"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
)

var (
	maxIdleConns    = flag.Int("db.max_idle_conns", 1, "Maximum Amount of Idle Database connections, -1 means the stdlib default")
	maxOpenConns    = flag.Int("db.max_open_conns", 5, "Maximum Amount of Open Database connections, -1 means the stdlib default")
	connMaxLifetime = flag.Duration("db.conn_max_lifetime", 30*time.Minute, "Maximum Database Connection Lifetime, -1ns means the stdlib default")
)

// ConfigurableDB contains methods for configuring a database.
type ConfigurableDB interface {
	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
	SetConnMaxLifetime(time.Duration)
	Stats() sql.DBStats
}

// Configure Sets Connection Boundaries and adds db_stats monitoring to monkit.
func Configure(ctx context.Context, db ConfigurableDB, dbName string, mon *monkit.Scope) {
	if *maxIdleConns >= 0 {
		db.SetMaxIdleConns(*maxIdleConns)
	}
	if *maxOpenConns >= 0 {
		db.SetMaxOpenConns(*maxOpenConns)
	}
	if *connMaxLifetime >= 0 {
		db.SetConnMaxLifetime(*connMaxLifetime)
	}
	mon.Chain(monkit.StatSourceFunc(
		func(cb func(key monkit.SeriesKey, field string, val float64)) {
			monkit.StatSourceFromStruct(monkit.NewSeriesKey("db_stats").WithTag("db_name", dbName), db.Stats()).Stats(cb)
		}))
}
