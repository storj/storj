// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/sqliteutil"
)

func TestMigrateTablesToDatabase(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	srcDB := newMemDB(t)
	defer ctx.Check(srcDB.Close)
	destDB := newMemDB(t)
	defer ctx.Check(srcDB.Close)

	query := `
		CREATE TABLE bobby_jones(I Int);
		INSERT INTO bobby_jones VALUES (1);
	`

	execSQL(t, srcDB, query)
	// This table should be removed after migration
	execSQL(t, srcDB, "CREATE TABLE what(I Int);")

	err := sqliteutil.MigrateTablesToDatabase(ctx, srcDB, destDB, "bobby_jones")
	require.NoError(t, err)

	destSchema, err := sqliteutil.QuerySchema(destDB)
	require.NoError(t, err)

	destData, err := sqliteutil.QueryData(destDB, destSchema)
	require.NoError(t, err)

	snapshot, err := sqliteutil.LoadSnapshotFromSQL(query)
	require.NoError(t, err)

	require.Equal(t, snapshot.Schema, destSchema)
	require.Equal(t, snapshot.Data, destData)
}

func TestKeepTables(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db := newMemDB(t)
	defer ctx.Check(db.Close)

	table1SQL := `
		CREATE TABLE table_one(I int);
		INSERT INTO table_one VALUES(1);
	`

	table2SQL := `
		CREATE TABLE table_two(I int);
		INSERT INTO table_two VALUES(2);
	`

	execSQL(t, db, table1SQL)
	execSQL(t, db, table2SQL)

	err := sqliteutil.KeepTables(ctx, db, "table_one")
	require.NoError(t, err)

	schema, err := sqliteutil.QuerySchema(db)
	require.NoError(t, err)

	data, err := sqliteutil.QueryData(db, schema)
	require.NoError(t, err)

	snapshot, err := sqliteutil.LoadSnapshotFromSQL(table1SQL)
	require.NoError(t, err)

	require.Equal(t, snapshot.Schema, schema)
	require.Equal(t, snapshot.Data, data)
}

func execSQL(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}

func newMemDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	return db
}
