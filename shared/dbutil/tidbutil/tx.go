// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/retrydb"
	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

// batchGuardVar is the session variable a checked-write guard writes its failure
// message into. It is read back only when finalizing errors, to recover which
// check failed and the actual vs expected counts.
const batchGuardVar = "@btx_guard"

// batchGuardDO aborts the surrounding multi-statement (before COMMIT) when
// batchGuardVar is non-NULL. It is a DO statement, so it produces no result set
// and does not disturb the result set a trailing CommitWithQuery expects. The
// correlated subquery returns two rows — surfacing as a runtime "Subquery returns
// more than 1 row" error — exactly when the guard variable is set; referencing
// the variable keeps TiDB from folding the subquery away, so it is only evaluated
// at runtime.
const batchGuardDO = "DO (SELECT 1 FROM (SELECT 1) a WHERE " + batchGuardVar + " IS NOT NULL" +
	" UNION ALL SELECT 2 FROM (SELECT 1) b WHERE " + batchGuardVar + " IS NOT NULL)"

// errAfterCommit marks an error that arose after the transaction's COMMIT had
// been irrevocably dispatched — specifically from the scanAfterCommit callback of
// CommitWithQuery. WithTx must never retry fn for such an error: the commit
// cannot be undone, so re-running fn would double-apply its writes.
var errAfterCommit = errs.Class("tidbutil.Tx after commit")

// IsAfterCommit reports whether err arose after the transaction's COMMIT had
// been irrevocably dispatched. Callers wrapping WithTx in their own retry loop
// must not re-run the transaction for such errors: the commit cannot be
// undone, so re-running would double-apply its writes.
func IsAfterCommit(err error) bool { return errAfterCommit.Has(err) }

// TxOptions configures a Tx.
type TxOptions struct {
	// Isolation is the transaction isolation level folded into the BEGIN that
	// opens the transaction. The zero value (sql.LevelDefault) leaves the server
	// default in place and emits no SET TRANSACTION statement. WithTx uses
	// sql.LevelReadCommitted.
	Isolation sql.IsolationLevel
}

// Tx is a TiDB transaction tuned to minimize round trips. Unlike a plain
// *sql.Tx it:
//
//   - folds the isolation level and BEGIN into the first statement it sends,
//     rather than spending a round trip on each,
//   - buffers writes enqueued with EnqueueExec / EnqueueExecExpectAffectedCount and
//     dispatches them together with COMMIT in a single round trip — the
//     affected-row check is enforced by an in-statement guard, so it costs no
//     extra round trip either, and
//   - can fold COMMIT into a final query or exec via CommitWithQuery /
//     CommitWithExec, so the last operation does not cost an extra round trip.
//
// A Tx runs on one pinned connection and owns its own COMMIT/ROLLBACK; it
// must be used only within WithTx, which manages that connection and the
// retry loop. It is not safe for concurrent use.
type Tx struct {
	conn      tagsql.Conn
	prelude   string
	started   bool
	committed bool
	pending   []batchStatement
}

// batchStatement is a write enqueued by EnqueueExec / EnqueueExecExpectAffectedCount
// until commit time. When checked, the statement must affect exactly expectRows;
// desc labels it in the guard's failure message.
type batchStatement struct {
	statement  string
	args       []any
	checked    bool
	expectRows int64
	desc       string
}

// Name returns the driver name of the underlying database, always
// tagsql.TiDBName. It lets driver-dispatching helpers such as dx.Do detect that
// a Tx speaks to TiDB and pick the multi-statement batching transport. The
// isolation+BEGIN prelude is folded transparently into whichever statement those
// helpers issue first.
func (tx *Tx) Name() string { return tagsql.TiDBName }

// prefix returns the bytes that must precede the next statement Tx sends.
// On the first call it returns the isolation+BEGIN prelude and marks the
// transaction started; afterwards it returns "".
func (tx *Tx) prefix() string {
	if tx.started {
		return ""
	}
	tx.started = true
	return tx.prelude
}

// ExecContext runs query within the transaction.
//
// On the first statement sent on the connection it folds the isolation+BEGIN
// prelude in front of query, opening the transaction in the same round trip.
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.conn.ExecContext(ctx, tx.prefix()+query, args...)
}

// QueryContext runs query within the transaction.
//
// On the first statement sent on the connection it folds the isolation+BEGIN
// prelude in front of query, opening the transaction in the same round trip.
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (tagsql.Rows, error) {
	// When the prelude is folded in, the MySQL driver fast-forwards past the
	// OK-packets of the leading SET/BEGIN to the query's own result set.
	return tx.conn.QueryContext(ctx, tx.prefix()+query, args...)
}

// QueryRowContext runs query within the transaction and returns the first row.
//
// On the first statement sent on the connection it folds the isolation+BEGIN
// prelude in front of query, opening the transaction in the same round trip.
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return tx.conn.QueryRowContext(ctx, tx.prefix()+query, args...)
}

// EnqueueExec buffers a write to be dispatched together with COMMIT in a single
// round trip. Enqueued statements always run last, after every direct Exec/Query
// call, so only enqueue writes that no later read in the transaction depends on.
func (tx *Tx) EnqueueExec(statement string, args ...any) {
	if tx.committed {
		panic("tidbutil.Tx already committed")
	}
	tx.pending = append(tx.pending, batchStatement{statement: statement, args: args})
}

// EnqueueExecExpectAffectedCount is like EnqueueExec but the enqueued statement must affect
// exactly expectRows rows; otherwise the commit is aborted and rolled back and
// the error names desc with the actual and expected counts. The check is a guard
// folded into the same multi-statement as COMMIT, so a checked write costs no
// extra round trip.
//
// Multiple checked writes are each enforced independently, but if more than one
// fails only the first (in enqueue order) is named in the error.
func (tx *Tx) EnqueueExecExpectAffectedCount(expectRows int64, desc string, statement string, args ...any) {
	if tx.committed {
		panic("tidbutil.Tx already committed")
	}
	tx.pending = append(tx.pending, batchStatement{
		statement:  statement,
		args:       args,
		checked:    true,
		expectRows: expectRows,
		desc:       desc,
	})
}

// buildFinal consumes the buffered writes (and the isolation+BEGIN prelude, if
// not yet emitted) into a SQL fragment ready to have a trailing statement and
// COMMIT appended. Checked writes (EnqueueExecExpectAffectedCount) capture their affected-row
// count and contribute a guard that aborts the multi-statement before COMMIT on
// a mismatch; hasGuard reports whether such a guard was emitted, so the caller
// knows the failure message can be read back. The returned args bind the write
// placeholders followed by the guard's desc placeholders, matching the SQL order.
//
// writeIndexes[k] is the 0-based position of the k-th enqueued write's statement
// within the folded multi-statement. The mysql driver returns one
// AllRowsAffected / AllLastInsertIds entry per statement, so these indexes let a
// raw-Exec commit (CommitWithResults) map results back to each EnqueueExec call —
// accounting for the prelude, guard reset, per-check SET, CASE and DO statements
// that occupy their own slots.
func (tx *Tx) buildFinal() (head string, args []any, hasGuard bool, writeIndexes []int) {
	for _, p := range tx.pending {
		if p.checked {
			hasGuard = true
			break
		}
	}

	var sb strings.Builder
	prelude := tx.prefix()
	sb.WriteString(prelude)
	// Each ";"-terminated statement produces one driver result entry. The prelude
	// is 0, 1 (BEGIN) or 2 (SET ISOLATION; BEGIN) statements; count its semicolons
	// so the indexes below line up with AllRowsAffected.
	stmtIndex := strings.Count(prelude, ";")
	if hasGuard {
		// Reset in case this pooled connection carries a value from an earlier use.
		sb.WriteString("SET " + batchGuardVar + "=NULL;")
		stmtIndex++
	}

	writeArgs := make([]any, 0, len(tx.pending))
	writeIndexes = make([]int, 0, len(tx.pending))
	var descArgs []any
	var guardCase strings.Builder
	checks := 0
	for _, p := range tx.pending {
		sb.WriteString(p.statement)
		sb.WriteByte(';')
		writeArgs = append(writeArgs, p.args...)
		writeIndexes = append(writeIndexes, stmtIndex)
		stmtIndex++
		if p.checked {
			fmt.Fprintf(&sb, "SET @c%d=ROW_COUNT();", checks)
			stmtIndex++
			fmt.Fprintf(&guardCase, "WHEN @c%d<>%d THEN CONCAT(?,' affected ',@c%d,', expected %d') ",
				checks, p.expectRows, checks, p.expectRows)
			descArgs = append(descArgs, p.desc)
			checks++
		}
	}
	tx.pending = nil

	if hasGuard {
		sb.WriteString("SET " + batchGuardVar + "=CASE ")
		sb.WriteString(guardCase.String())
		sb.WriteString("ELSE NULL END;")
		sb.WriteString(batchGuardDO)
		sb.WriteByte(';')
	}

	return sb.String(), append(writeArgs, descArgs...), hasGuard, writeIndexes
}

// finalizeErr converts an error from the finalizing round trip into the most
// specific error available. When the batch carried a guard that tripped, the
// failure message it stored in the session variable is read back — the variable
// survives the aborted statement on the still-open transaction — and returned;
// otherwise the original error (for example a enqueued write that failed
// outright) is returned.
func (tx *Tx) finalizeErr(ctx context.Context, hasGuard bool, execErr error) error {
	if hasGuard {
		var msg sql.NullString
		if err := tx.conn.QueryRowContext(ctx, "SELECT "+batchGuardVar).Scan(&msg); err == nil && msg.Valid {
			return errs.New("tidbutil.Tx: %s", msg.String)
		}
	}
	return errs.Wrap(execErr)
}

// commit dispatches the buffered writes, any affected-row guard, and COMMIT in a
// single round trip. When no statement ever opened the transaction and nothing
// was enqueued it is a no-op.
func (tx *Tx) commit(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if tx.committed {
		return nil
	}
	if !tx.started && len(tx.pending) == 0 {
		return nil
	}

	head, args, hasGuard, _ := tx.buildFinal()
	if _, err := tx.conn.ExecContext(ctx, head+"COMMIT", args...); err != nil {
		return tx.finalizeErr(ctx, hasGuard, err)
	}
	tx.committed = true
	return nil
}

// StatementResult holds the per-statement counts the mysql driver reports for a
// single enqueued write, returned by CommitWithResults.
type StatementResult struct {
	// RowsAffected is the number of rows the statement changed — matched rows when
	// the connection uses CLIENT_FOUND_ROWS.
	RowsAffected int64
	// LastInsertID is the AUTO_INCREMENT value the statement generated, or 0 when
	// it generated none.
	LastInsertID int64
}

// CommitWithResults flushes the enqueued writes together with COMMIT in a single
// round trip — exactly like the commit WithTx performs implicitly — and
// returns one StatementResult per EnqueueExec / EnqueueExecExpectAffectedCount call,
// in call order. Use it when a enqueued write needs its AUTO_INCREMENT id or
// affected-row count returned to Go.
//
// Unlike the implicit commit it runs the batch at the driver layer (Conn.Raw),
// because database/sql's Result wrapper hides the mysql driver's per-statement
// AllRowsAffected / AllLastInsertIds. Each enqueued statement must therefore be a
// single statement so its result occupies exactly one slot — the same assumption
// the affected-row guard already relies on for ROW_COUNT().
//
// CommitWithResults does not take a final read or write: results come only from
// the enqueued writes, so it has no CommitWithQuery/CommitWithExec equivalent
// (a query produces no driver result). Like the other Commit* methods, once it is
// called the transaction is committed and cannot be rolled back; call it last in
// fn and return its error.
func (tx *Tx) CommitWithResults(ctx context.Context) (_ []StatementResult, err error) {
	defer mon.Task()(&ctx)(&err)

	if tx.committed {
		return nil, errs.New("tidbutil.Tx already committed")
	}

	head, args, hasGuard, writeIndexes := tx.buildFinal()
	res, err := tx.execRaw(ctx, head+"COMMIT", args)
	if err != nil {
		return nil, tx.finalizeErr(ctx, hasGuard, err)
	}
	tx.committed = true

	results, err := mapStatementResults(res, writeIndexes)
	if err != nil {
		// The commit already happened; tag the error so WithTx does not retry.
		return nil, errAfterCommit.Wrap(err)
	}
	return results, nil
}

// Result extends sql.Result with the per-statement counts the mysql driver
// returns for multi-statement Exec calls.
//
// database/sql wraps each driver.Result in an unexported driverResult that does
// not forward AllRowsAffected / AllLastInsertIds, so callers that need these
// must reach into the driver via Conn.Raw — which is what execRaw arranges.
type Result interface {
	sql.Result

	// AllRowsAffected returns one entry per statement in the order the
	// statements appeared in the Exec query.
	AllRowsAffected() []int64

	// AllLastInsertIds returns one entry per statement in the order the
	// statements appeared in the Exec query.
	AllLastInsertIds() []int64
}

// execRaw runs query (a folded multi-statement) at the driver layer via Conn.Raw
// so the returned Result exposes the mysql driver's per-statement AllRowsAffected
// / AllLastInsertIds, which database/sql's Result wrapper hides.
func (tx *Tx) execRaw(ctx context.Context, query string, args []any) (result Result, err error) {
	rawErr := tx.conn.Raw(ctx, func(driverConn any) error {
		execer, ok := driverConn.(driver.ExecerContext)
		if !ok {
			return errs.New("tidbutil.Tx: driver conn %T does not implement driver.ExecerContext", driverConn)
		}
		checker, _ := driverConn.(driver.NamedValueChecker)
		nvs, err := convertArgs(checker, args)
		if err != nil {
			return err
		}
		res, err := execer.ExecContext(ctx, query, nvs)
		if err != nil {
			return err
		}
		r, ok := res.(Result)
		if !ok {
			return errs.New("tidbutil.Tx: driver result %T does not expose AllRowsAffected/AllLastInsertIds", res)
		}
		result = r
		return nil
	})
	return result, rawErr
}

// mapStatementResults picks each enqueued write's result out of the driver's
// per-statement slices using the statement indexes from buildFinal.
func mapStatementResults(res Result, writeIndexes []int) ([]StatementResult, error) {
	affected := res.AllRowsAffected()
	insertIDs := res.AllLastInsertIds()
	out := make([]StatementResult, len(writeIndexes))
	for k, idx := range writeIndexes {
		if idx >= len(affected) || idx >= len(insertIDs) {
			return nil, errs.New("result index %d out of range (affected=%d insertIds=%d); a enqueued statement may contain more than one statement",
				idx, len(affected), len(insertIDs))
		}
		out[k] = StatementResult{RowsAffected: affected[idx], LastInsertID: insertIDs[idx]}
	}
	return out, nil
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

// CommitWithExec runs statement as the final write of the transaction and folds
// the enqueued writes, their guard, and COMMIT into the same round trip. This
// avoids a separate COMMIT round trip when the last operation is a write. Any
// error — a failed affected-row check, a write, or COMMIT — surfaces
// synchronously from this call.
//
// It deliberately returns no sql.Result: because COMMIT is appended after
// statement, the driver reports the result of COMMIT (zero rows affected), not
// of statement. Callers that need the affected-row count must run the statement
// via ExecContext, spending a separate COMMIT round trip.
//
// Like the other Commit* methods, once it is called the transaction is committed
// and cannot be rolled back; call it last in fn and return its error.
func (tx *Tx) CommitWithExec(ctx context.Context, statement string, args ...any) (err error) {
	defer mon.Task()(&ctx)(&err)

	if tx.committed {
		return errs.New("tidbutil.Tx already committed")
	}

	head, hargs, hasGuard, _ := tx.buildFinal()
	hargs = append(hargs, args...)
	if _, err := tx.conn.ExecContext(ctx, head+statement+";COMMIT", hargs...); err != nil {
		return tx.finalizeErr(ctx, hasGuard, err)
	}
	tx.committed = true
	return nil
}

// CommitWithQuery runs query as the final read of the transaction, folds the
// enqueued writes, their guard, and COMMIT into the same round trip, and passes
// the result set to scanAfterCommit. This avoids a separate COMMIT round trip when
// the last operation is a query.
//
// The guard runs before query, so a failed affected-row check (or a failed
// enqueued write) surfaces synchronously from this call. Tx owns the row
// lifecycle: scanAfterCommit should consume the rows via rows.Next, and Tx
// drains and closes them afterwards. Draining runs the trailing COMMIT, so a
// COMMIT error surfaces synchronously too. scanAfterCommit may be nil to run query
// purely for effect.
//
// As the name says, scanAfterCommit runs against an already-committed
// transaction: once query has been dispatched the COMMIT is queued in the same
// stream and cannot be undone. An error it returns therefore does not roll the
// transaction back; it is wrapped as errAfterCommit so WithTx returns it as-is
// without retrying fn, since the commit is already durable and re-running fn
// would double-apply its writes.
func (tx *Tx) CommitWithQuery(ctx context.Context, query string, args []any, scanAfterCommit func(rows tagsql.Rows) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if tx.committed {
		return errs.New("tidbutil.Tx already committed")
	}

	head, hargs, hasGuard, _ := tx.buildFinal()
	hargs = append(hargs, args...)
	rows, err := tx.conn.QueryContext(ctx, head+query+";COMMIT", hargs...)
	if err != nil {
		// A enqueued write or its guard failed before the query; the transaction
		// is still open and will be rolled back by WithTx.
		return tx.finalizeErr(ctx, hasGuard, err)
	}
	// The query ran and COMMIT is queued behind it in the same stream: it runs as
	// the rows are drained below and cannot be rolled back, so mark the
	// transaction finalized regardless of what scanAfterCommit does.
	tx.committed = true
	defer func() { err = errs.Combine(err, errs.Wrap(rows.Close())) }()

	if scanAfterCommit != nil {
		if err := scanAfterCommit(rows); err != nil {
			// The commit is already irrevocable; tag the error so WithTx
			// returns it without retrying fn against the committed writes.
			return errAfterCommit.Wrap(err)
		}
	}
	// Advance past the query's result set so the trailing COMMIT executes and any
	// COMMIT error surfaces here instead of being swallowed by Close.
	for rows.NextResultSet() {
	}
	return errs.Wrap(rows.Err())
}

// ScanFirstRow builds a scanAfterCommit callback for CommitWithQuery that scans
// exactly one row into dest, walking past any leading empty result sets — for
// example the OK-packet of an UPDATE or INSERT that precedes the SELECT in the
// same multi-statement query. It returns sql.ErrNoRows when no result set
// yields a row.
//
// It deliberately neither drains the trailing result sets nor closes rows
// (unlike dx.ScanFirstRow, which does both): CommitWithQuery owns that
// lifecycle and runs the folded COMMIT once the callback returns, so the
// COMMIT's error surfaces through CommitWithQuery's own (retryable) path rather
// than being captured here as a post-commit error.
//
// Usage:
//
//	err := tx.CommitWithQuery(ctx, "UPDATE ...; SELECT ...", args, tidbutil.ScanFirstRow(&dest))
func ScanFirstRow(dest ...any) func(rows tagsql.Rows) error {
	return func(rows tagsql.Rows) error {
		for {
			if rows.Next() {
				break
			}
			if !rows.NextResultSet() {
				if err := rows.Err(); err != nil {
					return err
				}
				return sql.ErrNoRows
			}
		}
		return rows.Scan(dest...)
	}
}

// rollback aborts the transaction. It is a no-op when no statement opened one or
// when the transaction has already committed. Because a failed commit batch
// ("writes;COMMIT") stops before COMMIT and leaves the transaction open on the
// pinned connection, rollback is also issued after a commit error to avoid
// returning a dangling transaction to the pool.
func (tx *Tx) rollback(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !tx.started || tx.committed {
		return nil
	}
	if _, err := tx.conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// WithTx runs fn inside a TiDB Tx at READ COMMITTED isolation. See
// WithTxOptions for the general form and the retry/commit semantics.
func WithTx(ctx context.Context, db tagsql.DB, fn func(ctx context.Context, tx *Tx) error) error {
	return WithTxOptions(ctx, db, TxOptions{Isolation: sql.LevelReadCommitted}, fn)
}

// WithTxOptions runs fn inside a TiDB Tx configured by opts. If fn
// returns nil the enqueued writes and COMMIT are dispatched (see Tx for how
// many round trips that takes); if fn returns an error, or the commit fails, the
// transaction is rolled back.
//
// On retryable errors (TiDB write conflicts, InnoDB deadlocks, …) the whole fn
// is retried up to 10 times within a 5-minute budget — matching txutil.WithTx.
// Each attempt acquires a fresh connection from the pool. fn must be idempotent
// across retries: any side effects outside the transaction may run more than once.
func WithTxOptions(ctx context.Context, db tagsql.DB, opts TxOptions, fn func(ctx context.Context, tx *Tx) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if db.Name() != tagsql.TiDBName {
		return errs.New("tidbutil.WithTx requires a TiDB connection, got %q", db.Name())
	}

	prelude, err := txPrelude(opts.Isolation)
	if err != nil {
		return err
	}

	ctx = txutil.WithInsideTx(ctx)

	start := time.Now()
	for attempt := 0; ; attempt++ {
		txErr := runTxOnce(ctx, db, prelude, fn)
		if txErr == nil {
			mon.IntVal("batch_tx_retries").Observe(int64(attempt))
			return nil
		}
		// An error tagged errAfterCommit must not retry even if it looks retryable:
		// COMMIT has already been dispatched (see errAfterCommit for why).
		if errAfterCommit.Has(txErr) || !retrydb.ShouldRetry(txErr) {
			mon.IntVal("batch_tx_retries").Observe(int64(attempt))
			return txErr
		}
		if dur := time.Since(start); dur >= 5*time.Minute || attempt >= 10 {
			mon.IntVal("batch_tx_retries").Observe(int64(attempt))
			return errs.Combine(txErr, errs.New("unable to retry: duration:%v attempts:%d", dur, attempt+1))
		}
	}
}

// txPrelude builds the statement folded in front of the first query a
// Tx sends. SET TRANSACTION ISOLATION LEVEL (with no SESSION/GLOBAL scope)
// applies only to the transaction the following BEGIN starts.
func txPrelude(level sql.IsolationLevel) (string, error) {
	name, err := isolationLevelName(level)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "BEGIN;", nil
	}
	return "SET TRANSACTION ISOLATION LEVEL " + name + ";BEGIN;", nil
}

// isolationLevelName maps a sql.IsolationLevel to its SQL spelling. The default
// level returns "", meaning no SET TRANSACTION statement is emitted.
func isolationLevelName(level sql.IsolationLevel) (string, error) {
	switch level {
	case sql.LevelDefault:
		return "", nil
	case sql.LevelReadUncommitted:
		return "READ UNCOMMITTED", nil
	case sql.LevelReadCommitted:
		return "READ COMMITTED", nil
	case sql.LevelRepeatableRead:
		return "REPEATABLE READ", nil
	case sql.LevelSerializable:
		return "SERIALIZABLE", nil
	default:
		return "", errs.New("tidbutil.WithTx: unsupported isolation level %q", level)
	}
}

// runTxOnce pins a connection, disables the driver's per-statement conflict
// retry on it (WithTx drives retries at the fn level instead), runs fn, and
// commits or rolls back. The returned error is the fn/commit error and may be
// inspected by WithTx to decide whether to retry.
func runTxOnce(ctx context.Context, db tagsql.DB, prelude string, fn func(ctx context.Context, tx *Tx) error) (err error) {
	var tx *Tx
	// Once a Commit* method has finalized the transaction, the commit is durable
	// and no later error may be reported as retryable, or WithTx would re-run fn
	// against the committed writes. This guard catches every post-commit error
	// source — fn's own work after a Commit* call, and the connection cleanup
	// deferred below — and tags it so WithTx returns it as-is. It is registered
	// first so it runs last, after those deferred cleanups have folded into err.
	defer func() {
		if tx != nil && tx.committed && err != nil && !errAfterCommit.Has(err) {
			err = errAfterCommit.Wrap(err)
		}
	}()

	conn, err := db.Conn(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	// The driver retries single statements on conflict only when no transaction
	// is active, tracked by an inTransaction flag that BeginTx flips. Tx
	// opens the transaction with a raw BEGIN, which does not flip it, so without
	// this the statements after the first would be retried mid-transaction.
	// Mark the pinned connection in-transaction for the whole Tx lifetime
	// and clear it before the connection returns to the pool.
	if err := setConnInTransaction(ctx, conn, true); err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, setConnInTransaction(ctx, conn, false)) }()

	tx = &Tx{conn: conn, prelude: prelude}

	// done guards the panic path: if neither commit nor rollback resolved the
	// transaction, roll it back so the connection does not return to the pool
	// with an open server-side transaction.
	done := false
	defer func() {
		if !done {
			_ = tx.rollback(ctx)
		}
	}()

	if fnErr := fn(ctx, tx); fnErr != nil {
		rbErr := tx.rollback(ctx)
		done = true
		return errs.Combine(fnErr, rbErr)
	}

	commitErr := tx.commit(ctx)
	if commitErr != nil {
		rbErr := tx.rollback(ctx)
		done = true
		return errs.Combine(commitErr, rbErr)
	}
	done = true
	return nil
}

// setConnInTransaction toggles the tidbConn.inTransaction flag on the pinned
// connection via Conn.Raw, controlling whether the driver retries single
// statements on conflict.
func setConnInTransaction(ctx context.Context, conn tagsql.Conn, active bool) error {
	return conn.Raw(ctx, func(driverConn any) error {
		c, ok := driverConn.(*tidbConn)
		if !ok {
			return errs.New("tidbutil.WithTx requires a *tidbConn driver connection, got %T", driverConn)
		}
		c.inTransaction = active
		return nil
	})
}
