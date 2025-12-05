// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil_test

import (
	"context"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/dbutil/sqliteutil"
	"storj.io/storj/shared/tagsql"
)

func TestMigrateTablesToDatabase(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	srcDB := openMemDB(ctx, t)
	defer ctx.Check(srcDB.Close)
	destDB := openMemDB(ctx, t)
	defer ctx.Check(srcDB.Close)

	query := `
		CREATE TABLE bobby_jones(I Int);
		INSERT INTO bobby_jones VALUES (1);
	`

	execSQL(ctx, t, srcDB, query)
	// This table should be removed after migration
	execSQL(ctx, t, srcDB, "CREATE TABLE what(I Int);")

	err := sqliteutil.MigrateTablesToDatabase(ctx, srcDB, destDB, "bobby_jones")
	require.NoError(t, err)

	destSchema, err := sqliteutil.QuerySchema(ctx, destDB)
	require.NoError(t, err)

	destData, err := sqliteutil.QueryData(ctx, destDB, destSchema)
	require.NoError(t, err)

	snapshot, err := sqliteutil.LoadSnapshotFromSQL(ctx, query)
	require.NoError(t, err)

	require.Equal(t, snapshot.Schema, destSchema)
	require.Equal(t, snapshot.Data, destData)
}

func TestKeepTables(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db := openMemDB(ctx, t)
	defer ctx.Check(db.Close)

	table1SQL := `
		CREATE TABLE table_one(I int);
		INSERT INTO table_one VALUES(1);
	`

	table2SQL := `
		CREATE TABLE table_two(I int);
		INSERT INTO table_two VALUES(2);
	`

	execSQL(ctx, t, db, table1SQL)
	execSQL(ctx, t, db, table2SQL)

	err := sqliteutil.KeepTables(ctx, db, "table_one")
	require.NoError(t, err)

	schema, err := sqliteutil.QuerySchema(ctx, db)
	require.NoError(t, err)

	data, err := sqliteutil.QueryData(ctx, db, schema)
	require.NoError(t, err)

	snapshot, err := sqliteutil.LoadSnapshotFromSQL(ctx, table1SQL)
	require.NoError(t, err)

	require.Equal(t, snapshot.Schema, schema)
	require.Equal(t, snapshot.Data, data)
}

func execSQL(ctx context.Context, t *testing.T, db tagsql.DB, query string, args ...interface{}) {
	_, err := db.ExecContext(ctx, query, args...)
	require.NoError(t, err)
}

func openMemDB(ctx context.Context, t *testing.T) tagsql.DB {
	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	return db
}
