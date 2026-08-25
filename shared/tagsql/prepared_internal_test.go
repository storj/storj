// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestPreparedCancelledCallerStillPrepares(t *testing.T) {
	ctx := context.Background()
	raw, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db := Wrap(raw)
	defer func() { require.NoError(t, db.Close()) }()

	_, err = db.ExecContext(ctx, "CREATE TABLE kv (k INT PRIMARY KEY)")
	require.NoError(t, err)

	const get Statement = "SELECT k FROM kv WHERE k = ?"
	entry := func() *preparedEntry { return db.(*sqlDB).prepared.get(db.(*sqlDB), get) }

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	require.NotNil(t, entry().acquire(cancelled), "prepare does not run on the caller's context")
	require.True(t, entry().retryAt.IsZero(), "caller cancellation must not back off")
}
