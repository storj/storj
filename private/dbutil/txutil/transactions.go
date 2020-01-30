// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package txutil provides safe transaction-encapsulation functions which have retry
// semantics as necessary.
package txutil

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

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
		err, rollbackErr := withTxOnce(ctx, db, txOpts, fn)
		if time.Since(start) < 5*time.Minute && i < 10 {
			if code := errCode(err); code == "CR000" || code == "40001" {
				mon.Event(fmt.Sprintf("transaction_retry_%d", i+1))
				continue
			}
		}
		mon.IntVal("transaction_retries").Observe(int64(i))
		return errs.Wrap(errs.Combine(err, rollbackErr))
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
