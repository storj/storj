// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/tempdb"
	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

func TestQueryRetry(t *testing.T) {
	// this will t.Skip() if no spanner database is configured via test settings
	connURL := dbtest.PickOrStartSpanner(t)
	ctx := testcontext.New(t)
	db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connURL, "testqueryretry", nil)
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	id := testrand.UUID()

	_, err = db.ExecContext(ctx, `CREATE TABLE foo (id BYTES(MAX), val INT64) PRIMARY KEY (id)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO foo (id, val) VALUES (?, 1)`, id)
	require.NoError(t, err)

	var group errgroup.Group
	var barrier sync.WaitGroup
	goroutines := 2
	barrier.Add(goroutines)

	expectFinalVal := 1
	loopCounts := make([]int, goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		group.Go(func() error {
			return txutil.WithTx(ctx, db, nil, func(ctx context.Context, tx tagsql.Tx) error {
				loopCounts[i]++
				var val int64
				err := tx.QueryRowContext(ctx, `SELECT val FROM foo WHERE id = ?`, id).Scan(&val)
				// do this the first time only; if the transaction is retried, skip this part.
				// also do this before the first return, so that an error doesn't lead to deadlock.
				if loopCounts[i] == 1 {
					barrier.Done()
				}
				if err != nil {
					return errs.Wrap(err)
				}
				// now, either val is 1 (no transactions have successfully updated the row yet)
				// or it should be >10 (at least one transaction has updated it) if we are
				// retrying the transaction.
				if loopCounts[i] == 1 {
					if val != 1 {
						return errs.New("expected val=1 first time through, but got val=%d", val)
					}
				} else {
					if val <= 10 {
						return errs.New("expected val>10 after a retry, but got val=%d", val)
					}
				}
				// wait until all goroutines have gotten past the barrier.Done() line
				barrier.Wait()
				// update the row after having read it in all separate transactions.
				// only one of them should succeed the first time; the rest should get a
				// conflict error and retry.
				_, err = tx.ExecContext(ctx, `UPDATE foo SET val = ? + ? WHERE id = ?`, val, i+10, id)
				return errs.Wrap(err)
			})
		})
		expectFinalVal += i + 10
	}

	err = group.Wait()
	if err != nil {
		t.Errorf("err from transaction group: %+v", err)
		t.FailNow()
	}

	// verify that all transactions completed successfully
	var finalVal int64
	err = db.QueryRowContext(ctx, `SELECT val FROM foo WHERE id = ?`, id).Scan(&finalVal)
	require.NoError(t, err)
	require.Equal(t, int64(expectFinalVal), finalVal)

	// verify that the conflict generation worked as intended.
	conflictGenerationWorked := false
	for _, count := range loopCounts {
		if count > 1 {
			// success
			conflictGenerationWorked = true
			break
		}
	}
	require.True(t, conflictGenerationWorked, loopCounts)
}
