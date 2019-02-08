// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

import (
	"database/sql"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/migrate"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func TestCreate_Sqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	// should create table
	err = migrate.Create("example", migrate.NewSqliteDB(db, `CREATE TABLE example_table (id text)`))
	assert.NoError(t, err)

	// shouldn't create a new table
	err = migrate.Create("example", migrate.NewSqliteDB(db, `CREATE TABLE example_table (id text)`))
	assert.NoError(t, err)

	// should fail, because schema changed
	err = migrate.Create("example", migrate.NewSqliteDB(db, "CREATE TABLE example_table (id text, version int)"))
	assert.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = migrate.Create("conflict", migrate.NewSqliteDB(db, "CREATE TABLE example_table (id text, version int)"))
	assert.Error(t, err)
}

// this connstring is expected to work under the storj-test docker-compose instance
const defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"

var testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")

func TestCreate_Postgres(t *testing.T) {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	db, err := sql.Open("postgres", *testPostgres)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	// should create table
	err = migrate.Create("example", migrate.NewPostgresDB(db, "CREATE TABLE example_table (id text)"))
	assert.NoError(t, err)

	// shouldn't create a new table
	err = migrate.Create("example", migrate.NewPostgresDB(db, "CREATE TABLE example_table (id text)"))
	assert.NoError(t, err)

	// should fail, because schema changed
	err = migrate.Create("example", migrate.NewPostgresDB(db, "CREATE TABLE example_table (id text, version integer)"))
	assert.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = migrate.Create("conflict", migrate.NewPostgresDB(db, "CREATE TABLE example_table (id text, version integer)"))
	assert.Error(t, err)
}
