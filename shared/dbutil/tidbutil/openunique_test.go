// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/tidbutil"
	"storj.io/storj/shared/tagsql"
)

func TestOpenUnique(t *testing.T) {
	// PickTiDB returns the combined "<tidb-url>!!master=<postgres-url>" form
	// used by satellitedbtest; OpenUnique only needs the tidb:// portion.
	connstr, _, _ := strings.Cut(dbtest.PickTiDB(t), "!!master=")

	ctx := testcontext.New(t)

	prefix := "name#spaced/Test/DB"
	testDB, err := tidbutil.OpenUnique(ctx, connstr, prefix)
	require.NoError(t, err)

	require.Equal(t, tidbutil.DriverName, testDB.Driver)
	require.Equal(t, dbutil.TiDB, testDB.Implementation)

	// save before close so we can reopen against the same db name.
	driverCopy := testDB.Driver
	connStrCopy := testDB.ConnStr
	schemaName := testDB.Schema

	otherConn, err := tagsql.Open(ctx, driverCopy, connStrCopy, nil)
	require.NoError(t, err)
	defer ctx.Check(otherConn.Close)

	var currentDB string
	require.NoError(t, otherConn.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&currentDB))
	require.Equal(t, schemaName, currentDB)

	var count int
	require.NoError(t, otherConn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?`,
		schemaName,
	).Scan(&count))
	require.Equalf(t, 1, count, "expected 1 schema named %q, found %d", schemaName, count)

	require.NoError(t, testDB.Close())

	// reopen via the master DSN (no path) so we can confirm the schema is gone
	// even though otherConn's selected database no longer exists.
	masterConn, err := tagsql.Open(ctx, driverCopy, stripDBPath(t, connstr), nil)
	require.NoError(t, err)
	defer ctx.Check(masterConn.Close)

	require.NoError(t, masterConn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?`,
		schemaName,
	).Scan(&count))
	require.Equalf(t, 0, count, "expected schema %q to be dropped, found %d", schemaName, count)
}

func TestOpenUnique_RejectsNonTiDBScheme(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	_, err := tidbutil.OpenUnique(ctx, "mysql://root@localhost:4000/test", "prefix")
	require.Error(t, err)
	require.Contains(t, err.Error(), "tidb://")
}

// stripDBPath returns the input tidb:// URL with the database path cleared, so
// it can be used as a master connection that doesn't depend on the temporary
// database existing.
func stripDBPath(t *testing.T, connURL string) string {
	t.Helper()
	scheme, rest, ok := strings.Cut(connURL, "://")
	require.True(t, ok, "URL %q missing scheme separator", connURL)
	host, query, hasQuery := strings.Cut(rest, "?")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	out := scheme + "://" + host + "/"
	if hasQuery {
		out += "?" + query
	}
	return out
}
