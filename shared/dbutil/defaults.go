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

// ConnParams defines connection parameters.
type ConnParams struct {
	ConnMaxLifetime time.Duration `help:"Maximum Database Connection Lifetime, -1ns means the stdlib default" default:"30m"`
	MaxIdleConns    int           `help:"Maximum Amount of Idle Database connections, -1 means the stdlib default" default:"1"`
	MaxOpenConns    int           `help:"Maximum Amount of Open Database connections, -1 means the stdlib default" default:"5"`
}

// LegacyParameters returns the legacy parameters, set by pure flag package.
func LegacyParameters() *ConnParams {
	return &ConnParams{
		ConnMaxLifetime: *connMaxLifetime,
		MaxIdleConns:    *maxIdleConns,
		MaxOpenConns:    *maxOpenConns,
	}
}

// ConfigureParameters Sets Connection Boundaries.
func ConfigureParameters(db ConfigurableDB, params *ConnParams, dbName string, mon *monkit.Scope) {
	if params == nil {
		params = LegacyParameters()
	}
	if params.MaxIdleConns >= 0 {
		db.SetMaxIdleConns(params.MaxIdleConns)
	}
	if params.MaxOpenConns >= 0 {
		db.SetMaxOpenConns(params.MaxOpenConns)
	}
	if params.ConnMaxLifetime >= 0 {
		db.SetConnMaxLifetime(params.ConnMaxLifetime)
	}
	if mon != nil {
		mon.Chain(monkit.StatSourceFunc(
			func(cb func(key monkit.SeriesKey, field string, val float64)) {
				monkit.StatSourceFromStruct(monkit.NewSeriesKey("db_stats").WithTag("db_name", dbName), db.Stats()).Stats(cb)
			}))
	}
}

// Configure Sets Connection Boundaries and adds db_stats monitoring to monkit.
func Configure(ctx context.Context, db ConfigurableDB, dbName string, mon *monkit.Scope) {
	ConfigureParameters(db, LegacyParameters(), dbName, mon)
}
