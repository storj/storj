// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

import (
	"strconv"
	"testing"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/dbutil/tempdb"
	"storj.io/storj/private/migrate"
	"storj.io/storj/private/tagsql"
)

func TestCreate_Sqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	// should create table
	err = migrate.Create(ctx, "example", &sqliteDB{db, "CREATE TABLE example_table (id text)"})
	require.NoError(t, err)

	// shouldn't create a new table
	err = migrate.Create(ctx, "example", &sqliteDB{db, "CREATE TABLE example_table (id text)"})
	require.NoError(t, err)

	// should fail, because schema changed
	err = migrate.Create(ctx, "example", &sqliteDB{db, "CREATE TABLE example_table (id text, version int)"})
	require.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = migrate.Create(ctx, "conflict", &sqliteDB{db, "CREATE TABLE example_table (id text, version int)"})
	require.Error(t, err)
}

func TestCreate(t *testing.T) {
	pgtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, connstr, "create-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { assert.NoError(t, db.Close()) }()

		// should create table
		err = migrate.Create(ctx, "example", &postgresDB{db.DB, "CREATE TABLE example_table (id text)"})
		require.NoError(t, err)

		// shouldn't create a new table
		err = migrate.Create(ctx, "example", &postgresDB{db.DB, "CREATE TABLE example_table (id text)"})
		require.NoError(t, err)

		// should fail, because schema changed
		err = migrate.Create(ctx, "example", &postgresDB{db.DB, "CREATE TABLE example_table (id text, version integer)"})
		require.Error(t, err)

		// should fail, because of trying to CREATE TABLE with same name
		err = migrate.Create(ctx, "conflict", &postgresDB{db.DB, "CREATE TABLE example_table (id text, version integer)"})
		require.Error(t, err)
	})
}

type sqliteDB struct {
	tagsql.DB
	schema string
}

func (db *sqliteDB) Rebind(s string) string { return s }
func (db *sqliteDB) Schema() string         { return db.schema }

type postgresDB struct {
	tagsql.DB
	schema string
}

func (db *postgresDB) Rebind(sql string) string {
	out := make([]byte, 0, len(sql)+10)

	j := 1
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if ch != '?' {
			out = append(out, ch)
			continue
		}

		out = append(out, '$')
		out = append(out, strconv.Itoa(j)...)
		j++
	}

	return string(out)
}
func (db *postgresDB) Schema() string { return db.schema }
