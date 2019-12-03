// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/private/testcontext"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestNewCockroach(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	if *pgtest.CrdbConnStr == "" {
		t.Skip("Cockroachdb flag missing")
	}
	namespacedDBName := "name#spaced/Test/DB"
	testdb, err := satellitedbtest.NewCockroach(zap.L(), namespacedDBName)
	require.NoError(t, err)

	// assert new test db exists
	driver, source, err := dbutil.SplitConnstr(*pgtest.CrdbConnStr)
	require.NoError(t, err)

	db, err := dbx.Open(driver, source)
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	var exists *bool
	row := db.QueryRow(`SELECT EXISTS (
			SELECT datname FROM pg_catalog.pg_database WHERE lower(datname) = lower($1)
		);`, namespacedDBName,
	)
	err = row.Scan(&exists)
	require.NoError(t, err)
	assert.True(t, *exists)

	err = testdb.Close()
	require.NoError(t, err)

	// assert new test db was deleted
	row = db.QueryRow(`SELECT EXISTS (
			SELECT datname FROM pg_catalog.pg_database WHERE lower(datname) = lower($1)
		);`, namespacedDBName,
	)
	err = row.Scan(&exists)
	require.NoError(t, err)
	assert.False(t, *exists)
}
