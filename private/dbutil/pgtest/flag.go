// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgtest

import (
	"flag"
	"math/rand"
	"os"
	"strings"
	"testing"

	"storj.io/common/testcontext"
)

// We need to define this in a separate package due to https://golang.org/issue/23910.

// postgres is the test database connection string.
var postgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string (semicolon delimited for multiple)")

// cockroach is the test database connection string for CockroachDB
var cockroach = flag.String("cockroach-test-db", os.Getenv("STORJ_COCKROACH_TEST"), "CockroachDB test database connection string (semicolon delimited for multiple)")

// DefaultPostgres is expected to work under the storj-test docker-compose instance
const DefaultPostgres = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"

// DefaultCockroach is expected to work when a local cockroachDB instance is running
const DefaultCockroach = "cockroach://root@localhost:26257/master?sslmode=disable"

// Database defines a postgres compatible database.
type Database struct {
	Name string
	// Pick picks a connection string for the database and skips when it's missing.
	Pick func(t TB) string
}

// TB defines minimal interface required for Pick.
type TB interface {
	Skip(...interface{})
}

// Databases returns list of postgres compatible databases.
func Databases() []Database {
	return []Database{
		{Name: "Postgres", Pick: PickPostgres},
		{Name: "Cockroach", Pick: PickCockroach},
	}
}

// Run runs tests with all postgres compatible databases.
func Run(t *testing.T, test func(ctx *testcontext.Context, t *testing.T, connstr string)) {
	for _, db := range Databases() {
		db := db
		t.Run(db.Name, func(t *testing.T) {
			connstr := db.Pick(t)

			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			test(ctx, t, connstr)
		})
	}
}

// PickPostgres picks a random postgres database from flag.
func PickPostgres(t TB) string {
	if *postgres == "" {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + DefaultPostgres)
	}
	return pickRandom(*postgres)
}

// PickCockroach picks a random cockroach database from flag.
func PickCockroach(t TB) string {
	if *cockroach == "" {
		t.Skip("Cockroach flag missing, example: -cockroach-test-db=" + DefaultCockroach)
	}
	return pickRandom(*cockroach)
}

func pickRandom(dbstr string) string {
	values := strings.Split(dbstr, ";")
	if len(values) <= 1 {
		return dbstr
	}
	return values[rand.Intn(len(values))]
}
