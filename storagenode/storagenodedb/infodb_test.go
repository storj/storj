// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
)

func Run(t *testing.T, test func(t *testing.T, ctx context.Context, db *DB)) {
	log := zaptest.NewLogger(t)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := NewInMemory(log, ctx.Dir("storage"))
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NoError(t, db.CreateTables())

	test(t, ctx, db)
}
