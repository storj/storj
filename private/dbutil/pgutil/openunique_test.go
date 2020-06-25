// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/dbutil/tempdb"
	"storj.io/storj/private/tagsql"
)

func TestTempPostgresDB(t *testing.T) {
	connstr := pgtest.PickPostgres(t)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prefix := "name#spaced/Test/DB"
	testDB, err := tempdb.OpenUnique(ctx, connstr, prefix)
	require.NoError(t, err)

	// assert new test db exists and can be connected to again
	otherConn, err := tagsql.Open(testDB.Driver, testDB.ConnStr)
	require.NoError(t, err)
	defer ctx.Check(otherConn.Close)

	// verify the name matches expectation
	var name *string
	row := otherConn.QueryRowContext(ctx, `SELECT current_schema()`)
	err = row.Scan(&name)
	require.NoErrorf(t, err, "connStr=%q", testDB.ConnStr)
	require.NotNilf(t, name, "PG has no current_schema, which means the one we asked for doesn't exist. connStr=%q", testDB.ConnStr)
	require.Truef(t, strings.HasPrefix(*name, prefix), "Expected prefix of %q for current db name, but found %q", prefix, name)

	// verify there is an entry in pg_namespace with such a name
	var count int
	row = otherConn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_namespace WHERE nspname = current_schema`)
	err = row.Scan(&count)
	require.NoError(t, err)
	require.Equalf(t, 1, count, "Expected 1 schema with matching name, but counted %d", count)

	// close testDB but leave otherConn open
	err = testDB.Close()
	require.NoError(t, err)

	// assert new test schema was deleted
	row = otherConn.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_namespace WHERE nspname = current_schema`)
	err = row.Scan(&count)
	require.NoError(t, err)
	require.Equalf(t, 0, count, "Expected 0 schemas with matching name, but counted %d (deletion failure?)", count)
}
