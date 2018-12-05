// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
)

const (
	// this connstring is expected to work under the storj-test docker-compose instance
	defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
)

var (
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

func TestDatabase(t *testing.T) {
	testDrivers(t, func(ctx *testcontext.Context, t *testing.T, db *DB) {
		err := db.CreateTables()
		assert.NoError(t, err)
	})
}

func testDrivers(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *DB)) {
	t.Run("Sqlite", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// creating in-memory db and opening connection
		db, err := NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		fn(ctx, t, db)
	})

	t.Run("Postgres", func(t *testing.T) {
		if *testPostgres == "" {
			t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
		}

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := NewDB(*testPostgres)
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		fn(ctx, t, db)
	})
}
