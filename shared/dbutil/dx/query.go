// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package dx dispatches a sequence of SQL queries to a database, picking the
// most efficient transport supported by the underlying driver: pgx.Batch on
// pgx-backed databases (Postgres/CockroachDB), `;`-separated multi-statement
// queries on TiDB, and one round-trip per query elsewhere.
package dx

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/pgxutil"
	"storj.io/storj/shared/tagsql"
)

var mon = monkit.Package()

// Error is the default error class for the dx package.
var Error = errs.Class("dx")

// Rows is the minimal result-set view exposed to Query.Do callbacks. Both
// tagsql.Rows and pgx.Rows already satisfy this interface, so dx can pass
// either one through without an adapter. Lifecycle (Close, NextResultSet,
// driver-specific column metadata) is handled inside dx and is intentionally
// left off the interface.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// Query describes a single SQL statement and a function that processes the
// resulting rows. A zero-valued Query (empty Statement) is skipped, so callers
// can conditionally populate entries with an "if" block before passing them to
// Do.
type Query struct {
	// Statement is the SQL statement to execute. If empty, this Query is
	// skipped entirely.
	Statement string
	// Args are the bound parameters for Statement.
	Args []any
	// Do is invoked with the result set produced by Statement. It should
	// consume the rows with rows.Next as needed. If Do is nil the result set
	// is left untouched.
	Do func(rows Rows) error
}

// Do runs the non-empty queries against exec. Based on the driver name
// reported by exec, Do picks the most efficient batching transport: pgx.Batch
// on Postgres/CockroachDB, multi-statement `;`-joined queries on TiDB, and one
// round trip per query on everything else (e.g. Spanner). Both tagsql.DB and
// tagsql.Tx advertise their driver via Name(), so dispatch works inside or
// outside an open transaction.
//
// If all Query entries are empty, Do returns nil without touching exec.
func Do(ctx context.Context, exec tagsql.ExecQueryer, queries ...Query) (err error) {
	defer mon.Task()(&ctx)(&err)

	active := make([]Query, 0, len(queries))
	for _, q := range queries {
		if q.Statement != "" {
			active = append(active, q)
		}
	}
	if len(active) == 0 {
		return nil
	}

	name := driverName(exec)

	// pgx batch requires unwrapping to *pgx.Conn, which only works on tagsql.DB.
	if db, ok := exec.(tagsql.DB); ok {
		switch name {
		case tagsql.PostgresName, tagsql.CockroachName:
			return doPgxBatch(ctx, db, active)
		}
	}

	if name == tagsql.TiDBName {
		return doMultiStatement(ctx, exec, active)
	}
	return doSequential(ctx, exec, active)
}

// driverName returns the underlying driver name when exec exposes one. Both
// tagsql.DB and tagsql.Tx implement Name(); anything else returns the empty
// string.
func driverName(exec tagsql.ExecQueryer) string {
	if n, ok := exec.(interface{ Name() string }); ok {
		return n.Name()
	}
	return ""
}

func doPgxBatch(ctx context.Context, db tagsql.DB, queries []Query) error {
	return pgxutil.Conn(ctx, db, func(conn *pgx.Conn) (err error) {
		var batch pgx.Batch
		for _, q := range queries {
			batch.Queue(q.Statement, q.Args...)
		}

		results := conn.SendBatch(ctx, &batch)
		defer func() { err = errs.Combine(err, Error.Wrap(results.Close())) }()

		for _, q := range queries {
			rows, err := results.Query()
			if err != nil {
				return Error.Wrap(err)
			}
			if err := runPgxRows(q, rows); err != nil {
				return err
			}
		}
		return nil
	})
}

func runPgxRows(q Query, rows pgx.Rows) error {
	defer rows.Close()
	if q.Do != nil {
		if err := q.Do(rows); err != nil {
			return err
		}
	}
	return Error.Wrap(rows.Err())
}

func doMultiStatement(ctx context.Context, exec tagsql.ExecQueryer, queries []Query) (err error) {
	var sb strings.Builder
	args := make([]any, 0, len(queries)*2)
	for i, q := range queries {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString(q.Statement)
		args = append(args, q.Args...)
	}

	rows, err := exec.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, Error.Wrap(rows.Close())) }()

	for i, q := range queries {
		if i > 0 && !rows.NextResultSet() {
			return Error.Wrap(errs.Combine(rows.Err(), errs.New("missing result set %d", i)))
		}
		if q.Do == nil {
			continue
		}
		if err := q.Do(rows); err != nil {
			return err
		}
	}
	return Error.Wrap(rows.Err())
}

func doSequential(ctx context.Context, exec tagsql.ExecQueryer, queries []Query) error {
	for _, q := range queries {
		rows, err := exec.QueryContext(ctx, q.Statement, q.Args...)
		if err != nil {
			return Error.Wrap(err)
		}
		if err := runSequentialRows(q, rows); err != nil {
			return err
		}
	}
	return nil
}

func runSequentialRows(q Query, rows tagsql.Rows) (err error) {
	defer func() { err = errs.Combine(err, Error.Wrap(rows.Close())) }()
	if q.Do != nil {
		if err := q.Do(rows); err != nil {
			return err
		}
	}
	return Error.Wrap(rows.Err())
}
