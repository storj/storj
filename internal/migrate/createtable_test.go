package migrate

import (
	"database/sql"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	_ "github.com/mattn/go-sqlite3"
)

func TestCreateTable_Sqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)

	// should create table
	err = CreateTable(db, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// shouldn't create a new table
	err = CreateTable(db, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// should fail, because schema changed
	err = CreateTable(db, "example", "CREATE TABLE example_table (id text, version int)")
	assert.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = CreateTable(db, "conflict", "CREATE TABLE example_table (id text, version int)")
	assert.Error(t, err)
}

// this connstring is expected to work under the storj-test docker-compose instance
const defaultPostgresConn = "postgres://pointerdb:pg-secret-pass@test-postgres-pointerdb/pointerdb?sslmode=disable"

var testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRESKV_TEST"), "PostgreSQL test database connection string")

func TestCreateTable_Postgres(t *testing.T) {
	if *testPostgres == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
	}

	db, err := sql.Open("postgres", ":memory:")
	assert.NoError(t, err)

	// should create table
	err = CreateTable(db, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// shouldn't create a new table
	err = CreateTable(db, "example", "CREATE TABLE example_table (id text)")
	assert.NoError(t, err)

	// should fail, because schema changed
	err = CreateTable(db, "example", "CREATE TABLE example_table (id text, version integer)")
	assert.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = CreateTable(db, "conflict", "CREATE TABLE example_table (id text, version integer)")
	assert.Error(t, err)
}
