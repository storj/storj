// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
)

// PrecommitConstraint exposes precommitConstraint for testing.
type PrecommitConstraint precommitConstraint

// PrecommitConstraintResult exposes precommitConstraintResult for testing.
type PrecommitConstraintResult precommitConstraintResult

// StmtRow exposes stmtRow for testing.
type StmtRow stmtRow

// PrecommitConstraint exposes precommitConstraint for testing.
func (db *DB) PrecommitConstraint(ctx context.Context, opts PrecommitConstraint, tx StmtRow) (result PrecommitConstraintResult, err error) {
	r, err := db.precommitConstraint(ctx, precommitConstraint(opts), tx)
	return PrecommitConstraintResult(r), err
}

// PrecommitConstraintWithNonPendingResult exposes precommitConstraintWithNonPendingResult for testing.
type PrecommitConstraintWithNonPendingResult precommitConstraintWithNonPendingResult

// PrecommitDeleteUnversionedWithNonPending exposes precommitDeleteUnversionedWithNonPending for testing.
func (db *DB) PrecommitDeleteUnversionedWithNonPending(ctx context.Context, loc ObjectLocation, tx stmtRow) (result PrecommitConstraintWithNonPendingResult, err error) {
	r, err := db.precommitDeleteUnversionedWithNonPending(ctx, loc, tx)
	return PrecommitConstraintWithNonPendingResult(r), err
}
