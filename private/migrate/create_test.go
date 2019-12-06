// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

import (
	"database/sql"
	"strconv"
	"testing"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/private/dbutil/tempdb"
	"storj.io/storj/private/migrate"
)

func TestCreate_Sqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	// should create table
	err = migrate.Create("example", &sqliteDB{db, "CREATE TABLE example_table (id text)"})
	require.NoError(t, err)

	// shouldn't create a new table
	err = migrate.Create("example", &sqliteDB{db, "CREATE TABLE example_table (id text)"})
	require.NoError(t, err)

	// should fail, because schema changed
	err = migrate.Create("example", &sqliteDB{db, "CREATE TABLE example_table (id text, version int)"})
	require.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = migrate.Create("conflict", &sqliteDB{db, "CREATE TABLE example_table (id text, version int)"})
	require.Error(t, err)
}

func TestCreate_Postgres(t *testing.T) {
	if *pgtest.ConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", pgtest.DefaultConnStr)
	}
	testCreateGeneric(t, *pgtest.ConnStr)
}

func TestCreate_Cockroach(t *testing.T) {
	if *pgtest.CrdbConnStr == "" {
		t.Skip("Cockroach flag missing, example: -cockroach-test-db=" + pgtest.DefaultCrdbConnStr)
	}
	testCreateGeneric(t, *pgtest.CrdbConnStr)
}

func testCreateGeneric(t *testing.T, connStr string) {
	db, err := tempdb.OpenUnique(connStr, "create-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	// should create table
	err = migrate.Create("example", &postgresDB{db.DB, "CREATE TABLE example_table (id text)"})
	require.NoError(t, err)

	// shouldn't create a new table
	err = migrate.Create("example", &postgresDB{db.DB, "CREATE TABLE example_table (id text)"})
	require.NoError(t, err)

	// should fail, because schema changed
	err = migrate.Create("example", &postgresDB{db.DB, "CREATE TABLE example_table (id text, version integer)"})
	require.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = migrate.Create("conflict", &postgresDB{db.DB, "CREATE TABLE example_table (id text, version integer)"})
	require.Error(t, err)
}

type sqliteDB struct {
	*sql.DB
	schema string
}

func (db *sqliteDB) Rebind(s string) string { return s }
func (db *sqliteDB) Schema() string         { return db.schema }

type postgresDB struct {
	*sql.DB
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
