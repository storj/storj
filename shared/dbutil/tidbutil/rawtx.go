// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/retrydb"
	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

// Result extends sql.Result with the per-statement counts the mysql driver
// returns for multi-statement Exec calls.
//
// database/sql wraps each driver.Result in an unexported driverResult that does
// not forward AllRowsAffected / AllLastInsertIds, so callers that need these
// must reach into the driver via Conn.Raw — which is what WithRawTx arranges.
type Result interface {
	sql.Result

	// AllRowsAffected returns one entry per statement in the order the
	// statements appeared in the Exec query.
	AllRowsAffected() []int64

	// AllLastInsertIds returns one entry per statement in the order the
	// statements appeared in the Exec query.
	AllLastInsertIds() []int64
}

// RawTx is a TiDB transaction that operates at the driver layer so multi-
// statement Exec results expose AllRowsAffected / AllLastInsertIds via Result.
type RawTx interface {
	// ExecContext runs the given query (which may contain multiple ";"-separated
	// statements) within the transaction and returns a Result whose
	// AllRowsAffected reports a count for each statement.
	ExecContext(ctx context.Context, query string, args ...any) (Result, error)
}

// WithRawTx runs fn inside a TiDB transaction whose Exec results expose the
// mysql-driver-specific AllRowsAffected / AllLastInsertIds slices. If fn
// returns an error the transaction is rolled back; otherwise it is committed.
//
// On retryable errors (TiDB write conflicts, InnoDB deadlocks, …) the
// transaction is retried up to 10 times within a 5-minute budget — matching
// txutil.WithTx. Each attempt acquires a fresh connection from the pool so a
// connection left in a bad state by an earlier attempt does not poison the
// retry. fn must be idempotent across retries: any side effects outside the
// transaction may run more than once.
func WithRawTx(ctx context.Context, db tagsql.DB, fn func(ctx context.Context, tx RawTx) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if db.Name() != tagsql.TiDBName {
		return errs.New("tidbutil.WithRawTx requires a TiDB connection, got %q", db.Name())
	}

	ctx = txutil.WithInsideTx(ctx)

	start := time.Now()
	for attempt := 0; ; attempt++ {
		txErr := runRawTxOnce(ctx, db, fn)
		if txErr == nil {
			mon.IntVal("raw_tx_retries").Observe(int64(attempt))
			return nil
		}
		if !retrydb.ShouldRetry(txErr) {
			mon.IntVal("raw_tx_retries").Observe(int64(attempt))
			return txErr
		}
		if dur := time.Since(start); dur >= 5*time.Minute || attempt >= 10 {
			mon.IntVal("raw_tx_retries").Observe(int64(attempt))
			return errs.Combine(txErr, errs.New("unable to retry: duration:%v attempts:%d", dur, attempt+1))
		}
	}
}

// runRawTxOnce acquires a fresh connection, begins a driver-level transaction,
// runs fn, and commits or rolls back. The returned error is the fn error (or
// commit/begin error) and may be inspected by WithRawTx to decide whether to
// retry.
func runRawTxOnce(ctx context.Context, db tagsql.DB, fn func(ctx context.Context, tx RawTx) error) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	return conn.Raw(ctx, func(driverConn any) error {
		beginner, ok := driverConn.(driver.ConnBeginTx)
		if !ok {
			return errs.New("driver conn %T does not implement driver.ConnBeginTx", driverConn)
		}
		execer, ok := driverConn.(driver.ExecerContext)
		if !ok {
			return errs.New("driver conn %T does not implement driver.ExecerContext", driverConn)
		}
		checker, _ := driverConn.(driver.NamedValueChecker)

		dtx, err := beginner.BeginTx(ctx, driver.TxOptions{})
		if err != nil {
			return errs.Wrap(err)
		}

		// done is set once the tx has been resolved by Commit or an explicit
		// Rollback. The deferred Rollback covers the panic path where neither
		// happens — without it the connection would return to the pool with
		// an open server-side transaction.
		done := false
		defer func() {
			if !done {
				_ = dtx.Rollback()
			}
		}()

		fnErr := fn(ctx, &rawTx{execer: execer, checker: checker})
		if fnErr != nil {
			rbErr := dtx.Rollback()
			done = true
			if rbErr != nil {
				return errs.Combine(fnErr, errs.Wrap(rbErr))
			}
			return fnErr
		}
		commitErr := dtx.Commit()
		done = true
		return errs.Wrap(commitErr)
	})
}

// rawTx is the RawTx implementation backed by a driver-level conn whose
// transaction has already been started.
type rawTx struct {
	execer  driver.ExecerContext
	checker driver.NamedValueChecker
}

func (rt *rawTx) ExecContext(ctx context.Context, query string, args ...any) (Result, error) {
	nvs, err := convertArgs(rt.checker, args)
	if err != nil {
		return nil, err
	}
	res, err := rt.execer.ExecContext(ctx, query, nvs)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	r, ok := res.(Result)
	if !ok {
		return nil, errs.New("driver result %T does not expose AllRowsAffected/AllLastInsertIds", res)
	}
	return r, nil
}

// convertArgs translates []any into []driver.NamedValue, applying the driver's
// CheckNamedValue when available and falling back to driver.DefaultParameter-
// Converter — mirroring what database/sql does before calling ExecerContext.
func convertArgs(checker driver.NamedValueChecker, args []any) ([]driver.NamedValue, error) {
	nvs := make([]driver.NamedValue, len(args))
	for i, a := range args {
		nvs[i] = driver.NamedValue{Ordinal: i + 1, Value: a}
		if checker != nil {
			err := checker.CheckNamedValue(&nvs[i])
			if err == nil {
				continue
			}
			if !errors.Is(err, driver.ErrSkip) {
				return nil, errs.Wrap(err)
			}
		}
		v, err := driver.DefaultParameterConverter.ConvertValue(nvs[i].Value)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		nvs[i].Value = v
	}
	return nvs, nil
}
