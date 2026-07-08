// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"

	"storj.io/common/sync2"
	"storj.io/storj/shared/dbutil/tidbutil"
)

// tidbRetryVersionConflict re-runs fn a bounded number of times when it fails
// with a duplicate entry error. TiDB takes no gap locks, so writes that place
// an object at a freshly computed version (tidbGenerateNextVersion, a
// client-side timestamp, or nextVersion derived from a precommit query) can
// race with a concurrent writer inserting a row the reads could not lock: the
// loser fails with a duplicate primary key error (1062), which the shared
// retry layers treat as non-retryable. fn must only wrap operations whose
// primary key collision can solely be caused by such a version race, so that
// re-running fn recomputes the version and converges. It is a no-op wrapper
// for other databases, which never return MySQL error codes.
func tidbRetryVersionConflict(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	const maxAttempts = 10
	for attempt := range maxAttempts {
		err = fn(ctx)
		if err == nil || tidbutil.IsAfterCommit(err) || !tidbutil.IsDuplicateEntry(err) || ctx.Err() != nil {
			return err
		}
		mon.Event("tidb_version_conflict_retry")
		// Every loss means another writer committed at the contended location,
		// but the concurrent losers all recompute the same next version and can
		// keep colliding in lockstep; jittered backoff spreads them out.
		if !sync2.Sleep(ctx, time.Duration(rand.Int64N(int64(time.Millisecond)<<min(attempt, 5)))) {
			return err
		}
	}
	return err
}

// tidbInsertValuesClause returns the "(<col1>, ...) VALUES (?,?,...),(?,?,...),..."
// portion of a multi-row INSERT statement with rows placeholder tuples.
// rows must be > 0.
func tidbInsertValuesClause(cols []string, rows int) string {
	rowPlaceholder := "(" + tidbPlaceholders(len(cols)) + ")"
	return "(" + strings.Join(cols, ", ") + ") VALUES " +
		strings.Repeat(rowPlaceholder+",", rows-1) + rowPlaceholder
}

// tidbPlaceholders returns a comma-separated list of n "?" placeholders for
// use in IN(...) clauses, VALUES tuples, and similar. n must be > 0.
func tidbPlaceholders(n int) string {
	return strings.Repeat("?,", n-1) + "?"
}

// tidbBatchInsertQuery builds a multi-row INSERT statement of the form:
//
//	INSERT INTO <table> (<col1>, <col2>, ...) VALUES (?,?,...),(?,?,...),...
//
// with rows placeholder tuples. rows must be > 0.
func tidbBatchInsertQuery(table string, cols []string, rows int) string {
	return "INSERT INTO " + table + " " + tidbInsertValuesClause(cols, rows)
}

// tidbBatchInsertIgnoreQuery is like tidbBatchInsertQuery but emits
// INSERT IGNORE, which silently skips rows that would violate a unique or
// primary key constraint. Note that INSERT IGNORE also masks unrelated
// row-level errors (data conversion, NOT NULL violations, etc.) — only use
// where that is acceptable.
func tidbBatchInsertIgnoreQuery(table string, cols []string, rows int) string {
	return "INSERT IGNORE INTO " + table + " " + tidbInsertValuesClause(cols, rows)
}
