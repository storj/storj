// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package retrydb

import (
	"database/sql/driver"

	"github.com/spacemonkeygo/monkit/v3"
)

// StmtAll is the union of driver.Stmt interfaces a wrapped statement is
// expected to implement. Both cockroach (pgx-backed) and tidb (mysql-backed)
// drivers expose all three.
type StmtAll interface {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext
}

// ValuesToNamed adapts the deprecated driver.Value slice into the
// driver.NamedValue slice the *Context interfaces require.
func ValuesToNamed(args []driver.Value) []driver.NamedValue {
	named := make([]driver.NamedValue, len(args))
	for i, v := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return named
}

// RetryConflict invokes op, retrying on retryable errors as classified by
// ShouldRetry while inTx returns false. Retries inside a transaction
// are skipped — the caller (txutil.WithTx) replays business logic in that
// case. The mon scope is the caller's monkit package so the "needed_retry"
// event stays attributed to the driver that incurred the retry.
func RetryConflict[T any](mon *monkit.Scope, inTx func() bool, op func() (T, error)) (T, error) {
	result, err := op()
	for err != nil && !inTx() && ShouldRetry(err) {
		mon.Event("needed_retry")
		result, err = op()
	}
	return result, err
}
