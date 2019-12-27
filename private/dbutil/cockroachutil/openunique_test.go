// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/private/dbutil/tempdb"
)

func TestTempCockroachDB(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	if *pgtest.CrdbConnStr == "" {
		t.Skip("CockroachDB flag missing")
	}
	prefix := "name#spaced/Test/DB"
	testDB, err := tempdb.OpenUnique(*pgtest.CrdbConnStr, prefix)
	require.NoError(t, err)

	require.Equal(t, "cockroach", testDB.Driver)
	require.Equal(t, dbutil.Cockroach, testDB.Implementation)
	require.IsType(t, &cockroachutil.Driver{}, testDB.DB.Driver())

	// save these so we can close testDB down below and then still try connecting to the same place
	// (without requiring that the values stay intact in the testDB struct when we close it)
	driverCopy := testDB.Driver
	connStrCopy := testDB.ConnStr

	// assert new test db exists and can be connected to again
	otherConn, err := sql.Open(driverCopy, connStrCopy)
	require.NoError(t, err)
	defer ctx.Check(otherConn.Close)

	// verify the name matches expectation
	var dbName string
	row := otherConn.QueryRow(`SELECT current_database()`)
	err = row.Scan(&dbName)
	require.NoError(t, err)
	require.Truef(t, strings.HasPrefix(dbName, prefix), "Expected prefix of %q for current db name, but found %q", prefix, dbName)

	// verify there is a db with such a name
	var count int
	row = otherConn.QueryRow(`SELECT COUNT(*) FROM pg_database WHERE datname = current_database()`)
	err = row.Scan(&count)
	require.NoError(t, err)
	require.Equalf(t, 1, count, "Expected 1 DB with matching name, but counted %d", count)

	// close testDB
	err = testDB.Close()
	require.NoError(t, err)

	// make a new connection back to the master connstr just to check that the our temp db
	// really was dropped
	plainDBConn, err := sql.Open("cockroach", *pgtest.CrdbConnStr)
	require.NoError(t, err)
	defer ctx.Check(plainDBConn.Close)

	// assert new test db was deleted (we expect this connection to keep working, even though its
	// database was deleted out from under it!)
	row = plainDBConn.QueryRow(`SELECT COUNT(*) FROM pg_database WHERE datname = $1`, dbName)
	err = row.Scan(&count)
	require.NoError(t, err)
	require.Equalf(t, 0, count, "Expected 0 DB with matching name, but counted %d (deletion failure?)", count)
}
