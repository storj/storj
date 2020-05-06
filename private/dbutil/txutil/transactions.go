// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package txutil provides safe transaction-encapsulation functions which have retry
// semantics as necessary.
package txutil

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/private/tagsql"
)

var mon = monkit.Package()

// WithTx starts a transaction on the given sql.DB. The transaction is started in the appropriate
// manner, and will be restarted if appropriate. While in the transaction, fn is called with a
// handle to the transaction in order to make use of it. If fn returns an error, the transaction
// is rolled back. If fn returns nil, the transaction is committed.
//
// If fn has any side effects outside of changes to the database, they must be idempotent! fn may
// be called more than one time.
func WithTx(ctx context.Context, db tagsql.DB, txOpts *sql.TxOptions, fn func(context.Context, tagsql.Tx) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	start := time.Now()

	for i := 0; ; i++ {
		var retryErr error
		err, rollbackErr := withTxOnce(ctx, db, txOpts, fn)
		// if we had any error, check to see if we should retry.
		if err != nil || rollbackErr != nil {
			// we will only retry if we have enough resources (duration and count).
			if dur := time.Since(start); dur < 5*time.Minute && i < 10 {
				// even though the resources (duration and count) allow us to issue a retry,
				// we only should if the error claims we should.
				if code := errCode(err); code == "CR000" || code == "40001" {
					continue
				}
			} else {
				// we aren't issuing a retry due to resources (duration and count), so
				// include a retry error in the output so that we know something is wrong.
				retryErr = errs.New("unable to retry: duration:%v attempts:%d", dur, i)
			}
		}
		mon.IntVal("transaction_retries").Observe(int64(i))
		return errs.Wrap(errs.Combine(err, rollbackErr, retryErr))
	}
}

// withTxOnce creates a transaction, ensures that it is eventually released (commit or rollback)
// and passes it to the provided callback. It does not handle retries or anything, delegating
// that to callers.
func withTxOnce(ctx context.Context, db tagsql.DB, txOpts *sql.TxOptions, fn func(context.Context, tagsql.Tx) error) (err, rollbackErr error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := db.BeginTx(ctx, txOpts)
	if err != nil {
		return errs.Wrap(err), nil
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			rollbackErr = tx.Rollback()
		}
	}()

	return fn(ctx, tx), nil
}

// errCode returns the error code associated with any postgres error in the chain of
// errors walked by unwrapping.
func errCode(err error) (code string) {
	errs.IsFunc(err, func(err error) bool {
		if pgerr, ok := err.(*pq.Error); ok {
			code = string(pgerr.Code)
			return true
		}
		return false
	})
	return code
}
