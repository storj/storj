// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"database/sql"
	"flag"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func TestCreateTable_Sqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	// should create table
	err = CreateTable(db, rebindSqlite, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// shouldn't create a new table
	err = CreateTable(db, rebindSqlite, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// should fail, because schema changed
	err = CreateTable(db, rebindSqlite, "example", "CREATE TABLE example_table (id text, version int)")
	assert.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = CreateTable(db, rebindSqlite, "conflict", "CREATE TABLE example_table (id text, version int)")
	assert.Error(t, err)
}

// this connstring is expected to work under the storj-test docker-compose instance
const defaultPostgresConn = "postgres://pointerdb:pg-secret-pass@test-postgres-pointerdb/pointerdb?sslmode=disable"

var testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRESKV_TEST"), "PostgreSQL test database connection string")

func TestCreateTable_Postgres(t *testing.T) {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	db, err := sql.Open("postgres", *testPostgres)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	// should create table
	err = CreateTable(db, rebindPostgres, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// shouldn't create a new table
	err = CreateTable(db, rebindPostgres, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// should fail, because schema changed
	err = CreateTable(db, rebindPostgres, "example", "CREATE TABLE example_table (id text, version integer)")
	assert.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = CreateTable(db, rebindPostgres, "conflict", "CREATE TABLE example_table (id text, version integer)")
	assert.Error(t, err)
}

func rebindSqlite(s string) string { return s }

func rebindPostgres(sql string) string {
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
