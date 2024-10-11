// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
)

// WithTx is a helper method which executes callback in transaction scope.
func WithTx(ctx context.Context, db tagsql.DB, cb func(ctx context.Context, tx tagsql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()
	return cb(ctx, tx)
}
