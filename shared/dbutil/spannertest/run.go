// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannertest

import (
	"context"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	databasepb "cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/common/testrand"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/spannerutil"
)

// Error is error class for this package.
var Error = errs.Class("spannertest")

// RunClient creates a new temporary spanner database, executes the ddls and finally connects a spanner client to it.
func RunClient(ctx context.Context, t *testing.T, ddls string, run func(t *testing.T, client *spanner.Client)) {
	connurl := dbtest.PickSpanner(t)

	params, err := spannerutil.ParseConnStr(connurl)
	require.NoError(t, err)

	params.Database += "_" + strings.ToLower(string(testrand.RandAlphaNumeric(8)))

	admin, err := database.NewDatabaseAdminClient(ctx, params.ClientOptions()...)
	require.NoError(t, err)
	defer func() { require.NoError(t, admin.Close()) }()

	req := &databasepb.CreateDatabaseRequest{
		Parent:          params.InstancePath(),
		DatabaseDialect: databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL,
		CreateStatement: "CREATE DATABASE `" + params.Database + "`",
	}

	for _, ddl := range strings.Split(ddls, ";") {
		if strings.TrimSpace(ddl) != "" {
			req.ExtraStatements = append(req.ExtraStatements, ddl)
		}
	}

	ddl, err := admin.CreateDatabase(ctx, req)
	require.NoError(t, err)

	_, err = ddl.Wait(ctx)
	require.NoError(t, err)

	defer func() {
		ctx, cancel := context.WithTimeout(context2.WithoutCancellation(ctx), 10*time.Second)
		defer cancel()

		err := admin.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
			Database: params.DatabasePath(),
		})
		require.NoError(t, err)
	}()

	client, err := spanner.NewClient(ctx, params.DatabasePath(), params.ClientOptions()...)
	require.NoError(t, err)
	defer client.Close()

	run(t, client)
}
