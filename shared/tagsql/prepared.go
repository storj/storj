// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
)

// Statement identifies a query that a DB prepares once and keeps for its own
// lifetime, so the server can serve it from its prepared plan cache. Declare
// one per hot query and run it through DB.Prepared:
//
//	const getObject tagsql.Statement = `SELECT ... WHERE id = ?`
//	...
//	err := db.Prepared(getObject).QueryRowContext(ctx, id).Scan(&object)
//
// A prepared statement costs a server-side handle on every pooled connection,
// so this is for the few queries worth it, not a cache for whatever text a
// caller passes.
type Statement string

// Prepared is a Statement bound to a DB. Every method runs the query, on the
// prepared statement when one is available and as plain text otherwise, so no
// call fails that the plain query would have served.
//
// That fallback re-runs the query, and a statement failing does not prove the
// server did not apply it, so the query must be idempotent: a SELECT, or a
// mutation that is safe to apply twice. Do not wrap INSERT ... RETURNING and
// friends in a Prepared.
type Prepared interface {
	QueryContext(ctx context.Context, args ...interface{}) (Rows, error)
	QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row
}

const (
	// preparedTimeout bounds a prepare, which runs detached from the caller.
	preparedTimeout = 10 * time.Second
	// preparedRetryInterval keeps a statement out of use after preparing or
	// using it failed: a server refusing to prepare is already under strain,
	// and a doomed prepare adds a round trip to every execution.
	preparedRetryInterval = time.Minute
	// preparedDropAfter is how many consecutive failures make the statement
	// itself the suspect. One is not: the query can fail for reasons of its
	// own on a healthy statement, and dropping then would keep the plan cache
	// off most of the time.
	preparedDropAfter = 3
)

var monPreparedPrepareError = mon.Meter("prepared_stmt_prepare_error")

// monPreparedFallback counts fallbacks to the plain query, tagged by the error
// type behind it: a statement the server refuses to re-prepare on a new
// connection and an ordinary query error need different responses.
func monPreparedFallback(err error) *monkit.Meter {
	return mon.Meter("prepared_stmt_fallback", monkit.NewSeriesTag("error", fmt.Sprintf("%T", err)))
}

// preparedCache holds the prepared statements of one DB.
type preparedCache struct {
	mu      sync.Mutex
	closed  bool
	entries map[Statement]*preparedEntry
}

type preparedEntry struct {
	statement Statement
	db        *sqlDB
	cache     *preparedCache

	// Guarded by cache.mu.
	stmt      Stmt
	preparing bool
	retryAt   time.Time
	failures  int
}

func (c *preparedCache) get(db *sqlDB, statement Statement) *preparedEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[Statement]*preparedEntry{}
	}
	e, ok := c.entries[statement]
	if !ok {
		e = &preparedEntry{statement: statement, db: db, cache: c}
		c.entries[statement] = e
	}
	return e
}

// close closes every prepared statement; the DB is closed afterwards.
func (c *preparedCache) close() error {
	c.mu.Lock()
	c.closed = true
	var stmts []Stmt
	for _, e := range c.entries {
		if e.stmt != nil {
			stmts = append(stmts, e.stmt)
			e.stmt = nil
		}
	}
	c.mu.Unlock()

	var group errs.Group
	for _, stmt := range stmts {
		group.Add(stmt.Close())
	}
	return group.Err()
}

func (e *preparedEntry) QueryContext(ctx context.Context, args ...interface{}) (Rows, error) {
	if stmt := e.acquire(ctx); stmt != nil {
		rows, err := stmt.QueryContext(ctx, args...)
		if err == nil {
			e.succeeded()
			return rows, nil
		}
		if ctx.Err() != nil {
			return nil, err
		}
		e.failed(err)
	}
	return e.db.QueryContext(ctx, string(e.statement), args...)
}

func (e *preparedEntry) QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row {
	if stmt := e.acquire(ctx); stmt != nil {
		row := stmt.QueryRowContext(ctx, args...)
		err := row.Err()
		if err == nil {
			e.succeeded()
			return row
		}
		if ctx.Err() != nil {
			return row
		}
		e.failed(err)
	}
	return e.db.QueryRowContext(ctx, string(e.statement), args...)
}

// acquire returns the prepared statement, preparing it on first use, or nil
// when the caller should run the plain query instead. At most one caller
// prepares at a time; the others do not queue behind it, since each waiting
// prepare would hold a pool connection while the server is slow to answer.
func (e *preparedEntry) acquire(ctx context.Context) Stmt {
	c := e.cache
	c.mu.Lock()
	if e.stmt != nil || c.closed || e.preparing || time.Now().Before(e.retryAt) {
		stmt := e.stmt
		c.mu.Unlock()
		return stmt
	}
	e.preparing = true
	c.mu.Unlock()

	// The prepare does not run on the caller's context: it must not spend the
	// caller's deadline, which the caller still needs for the query itself.
	prepareCtx, cancel := context2.WithRetimeout(ctx, preparedTimeout)
	defer cancel()
	stmt, err := e.db.PrepareContext(prepareCtx, string(e.statement))

	c.mu.Lock()
	defer c.mu.Unlock()
	e.preparing = false
	if err != nil {
		// Only preparedTimeout cancels prepareCtx, so every failure here --
		// the timeout included -- is the server's, and worth backing off from.
		monPreparedPrepareError.Mark(1)
		e.retryAt = time.Now().Add(preparedRetryInterval)
		return nil
	}
	if c.closed {
		_ = stmt.Close()
		return nil
	}
	e.stmt = stmt
	return stmt
}

func (e *preparedEntry) succeeded() {
	c := e.cache
	c.mu.Lock()
	e.failures = 0
	c.mu.Unlock()
}

// failed records a failure and, after preparedDropAfter in a row, drops the
// statement. database/sql prepares the statement again on every pooled
// connection it lands on and hands that failure to the caller, so a server
// refusing to prepare surfaces here rather than in acquire.
func (e *preparedEntry) failed(err error) {
	monPreparedFallback(err).Mark(1)

	c := e.cache
	c.mu.Lock()
	e.failures++
	var stmt Stmt
	if e.failures >= preparedDropAfter {
		stmt, e.stmt = e.stmt, nil
		e.failures = 0
		e.retryAt = time.Now().Add(preparedRetryInterval)
	}
	c.mu.Unlock()

	if stmt != nil {
		// Outside the mutex: Close waits for in-flight executions to return.
		_ = stmt.Close()
	}
}
