// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
)

// WithConn pins a single pooled connection for the duration of fn and ensures
// it is returned to the pool when fn returns. Use this when multiple statements
// must run on the same session — for example, to read back MySQL/TiDB's
// LAST_INSERT_ID() after an INSERT.
func WithConn(ctx context.Context, db tagsql.DB, fn func(context.Context, tagsql.Conn) error) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()
	return fn(ctx, conn)
}
