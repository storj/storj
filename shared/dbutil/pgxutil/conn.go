// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package pgxutil

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
)

var mon = monkit.Package()

// Error is default error class for the package.
var Error = errs.Class("pgxutil")

// Conn unwraps tagsql.DB to return the underlying raw *pgx.Conn.
func Conn(ctx context.Context, db tagsql.DB, fn func(conn *pgx.Conn) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := db.Conn(ctx)
	if err != nil {
		return Error.New("unable to get Conn: %w", err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	return conn.Raw(ctx, func(driverConn interface{}) (err error) {
		var pgxconn *pgx.Conn
		switch conn := driverConn.(type) {
		case interface{ StdlibConn() *stdlib.Conn }:
			pgxconn = conn.StdlibConn().Conn()
		case *stdlib.Conn:
			pgxconn = conn.Conn()
		default:
			return Error.New("invalid driver %T", driverConn)
		}

		return fn(pgxconn)
	})
}
