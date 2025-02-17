// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package retrydb

import (
	"errors"
	"io"
	"net"
	"syscall"

	"cloud.google.com/go/spanner"
	sqlspanner "github.com/googleapis/go-sql-spanner"
	pgxerrcode "github.com/jackc/pgerrcode"
	"github.com/spacemonkeygo/monkit/v3"
	"google.golang.org/grpc/codes"

	"storj.io/storj/shared/dbutil/pgutil/pgerrcode"
)

var (
	mon = monkit.Package()
)

const crdbRetry = "CR000"

// ShouldRetry indicates, given a query execution error, whether the query
// should be (and can safely be) retried.
func ShouldRetry(err error) bool {
	return shouldRetry(err, false)
}

// ShouldRetryIdempotent indicates, given a query execution error, whether an
// idempotent query should be (and can safely be) retried.
func ShouldRetryIdempotent(err error) bool {
	return shouldRetry(err, true)
}

// shouldRetry indicates, given a query execution error and whether the query
// can be expected to be idempotent, whether the query should be retried.
func shouldRetry(err error, isIdempotent bool) bool {
	if spanner.ErrCode(err) == codes.Aborted {
		// This is the primary way Spanner indicates that a transaction
		// should be retried.
		return true
	}

	if errors.Is(err, sqlspanner.ErrAbortedDueToConcurrentModification) {
		// This indicates that we should retry a transaction from the beginning
		// (go-sql-spanner tried to replay the transaction, but got some
		// different result from the first time through). It is safe to retry
		// as long as we reapply our business logic.
		return true
	}

	// * 40001 (SerializationFailure) occurs when a transaction has conflicted
	//   with another one and should be retried.
	// * 40P01 (DeadlockDetected) occurs when a deadlock is detected and
	//   resolved by canceling one of them.
	// * 57P01 (AdminShutdown) occurs when the administrator is shutting down a
	//   PostgreSQL instance (after which, presumably, it will be brought back
	//   up again). CockroachDB also uses this to indicate that a node has
	//   rejoined the cluster but is not ready to accept connections.
	// * CR000 (crdbRetry) was issued in place of SerializationFailure in older
	//   versions of CockroachDB.
	switch pgerrcode.FromError(err) {
	case crdbRetry, pgxerrcode.SerializationFailure, pgxerrcode.DeadlockDetected, pgxerrcode.AdminShutdown:
		return true
	case "":
		// not a PostgreSQL error; continue
	default:
		return false
	}

	// We don't retry in these situations (unless the query is marked as
	// safely idempotent) because it's possible the query has already been
	// executed and something went wrong before we received the result.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		mon.Event("db_error_eof")
		return isIdempotent
	}
	if errors.Is(err, syscall.ECONNRESET) {
		mon.Event("db_error_conn_reset")
		return isIdempotent
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		mon.Event("db_net_error")
		return isIdempotent
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		// In this case, we can be sure the query was not executed; we failed to
		// talk to the db at all.
		mon.Event("db_error_conn_refused")
		return true
	}

	return false
}
