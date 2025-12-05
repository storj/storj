// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannertest

import (
	"context"
	"testing"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// Error is error class for this package.
var Error = errs.Class("spannertest")

// RunClient creates a new temporary spanner database, executes the ddls and finally connects a spanner client to it.
func RunClient(ctx context.Context, t *testing.T, ddls string, run func(t *testing.T, client *spanner.Client)) {
	connstr := dbtest.PickOrStartSpanner(t)

	ephemeral, err := spannerutil.CreateEphemeralDB(ctx, connstr, t.Name(), spannerutil.MustSplitSQLStatements(ddls)...)
	require.NoError(t, err)
	defer func() { require.NoError(t, ephemeral.Close(ctx)) }()

	client, err := spanner.NewClient(ctx, ephemeral.Params.DatabasePath(), ephemeral.Params.ClientOptions()...)
	require.NoError(t, err)
	defer client.Close()

	run(t, client)
}
