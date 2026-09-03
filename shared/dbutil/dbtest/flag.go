// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbtest

import (
	"context"
	"flag"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"storj.io/common/testcontext"
)

// We need to define this in a separate package due to https://golang.org/issue/23910.

// postgres is the test database connection string.
var postgres = flag.String("postgres-test-db", os.Getenv("STORJ_TEST_POSTGRES"), "PostgreSQL test database connection string (semicolon delimited for multiple), \"omit\" is used to omit the tests from output")

// cockroach is the test database connection string for CockroachDB.
var cockroach = flag.String("cockroach-test-db", os.Getenv("STORJ_TEST_COCKROACH"), "CockroachDB test database connection string (semicolon delimited for multiple), \"omit\" is used to omit the tests from output")
var cockroachAlt = flag.String("cockroach-test-alt-db", os.Getenv("STORJ_TEST_COCKROACH_ALT"), "CockroachDB test database connection alternate string (semicolon delimited for multiple), \"omit\" is used to omit the tests from output")

// tidb is the test database connection string for TiDB.
var tidb = flag.String("tidb-test-db", os.Getenv("STORJ_TEST_TIDB"), "TiDB test database connection string (semicolon delimited for multiple), \"omit\" is used to omit the tests from output")

// DefaultPostgres is expected to work under the storj-test docker-compose instance.
const DefaultPostgres = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"

// DefaultCockroach is expected to work when a local cockroachDB instance is running.
const DefaultCockroach = "cockroach://root@localhost:26257/master?sslmode=disable"

// DefaultTiDB is expected to work when a local TiDB instance is running.
const DefaultTiDB = "tidb://root@localhost:4000/teststorj?parseTime=true!!master=postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"

// Database defines a postgres compatible database.
type Database struct {
	Name string
	Flag *string
	// Pick picks a connection string for the database and skips when it's missing.
	Pick func(t TB) string
}

// TB defines minimal interface required for Pick.
type TB interface {
	Cleanup(func())
	Context() context.Context
	Fatal(...any)
	Fatalf(format string, args ...any)
	Log(...any)
	Logf(string, ...any)
	Skip(...any)
}

// Databases returns list of postgres compatible databases.
func Databases() []Database {
	return []Database{
		{Name: "Postgres", Flag: postgres, Pick: PickPostgres},
		{Name: "Cockroach", Flag: cockroach, Pick: PickCockroach},
	}
}

// Run runs tests with all postgres compatible databases.
func Run(t *testing.T, test func(ctx *testcontext.Context, t *testing.T, connstr string)) {
	t.Parallel()
	for _, db := range Databases() {
		db := db
		if strings.EqualFold(*db.Flag, "omit") {
			continue
		}
		t.Run(db.Name, func(t *testing.T) {
			connstr := db.Pick(t)
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			test(ctx, t, connstr)
		})
	}
}

// PickPostgres picks one postgres database from flag.
func PickPostgres(t TB) string {
	if *postgres == "" || strings.EqualFold(*postgres, "omit") {
		t.Skip("Postgres flag missing, example: -postgres-test-db=" + DefaultPostgres)
	}
	return PickPostgresNoSkip()
}

// PickPostgresNoSkip picks one postgres database from flag, but doesn't autoskip.
func PickPostgresNoSkip() string {
	return pickNext(*postgres, &pickPostgres)
}

// PickCockroach picks one cockroach database from flag.
func PickCockroach(t TB) string {
	if *cockroach == "" || strings.EqualFold(*cockroach, "omit") {
		t.Skip("Cockroach flag missing, example: -cockroach-test-db=" + DefaultCockroach)
	}
	return PickCockroachNoSkip()
}

// PickCockroachNoSkip picks one cockroach database from flag, but doesn't autoskip.
func PickCockroachNoSkip() string {
	return pickNext(*cockroach, &pickCockroach)
}

// PickTiDB picks one TiDB database from flag.
func PickTiDB(t TB) string {
	if *tidb == "" || strings.EqualFold(*tidb, "omit") {
		t.Skip("TiDB flag missing, example: -tidb-test-db=" + DefaultTiDB)
	}
	return PickTiDBNoSkip()
}

// PickTiDBNoSkip picks one TiDB database from flag, but doesn't autoskip.
func PickTiDBNoSkip() string {
	return pickNext(*tidb, &pickTiDB)
}

// PickCockroachAlt picks an alternate cockroach database from flag.
//
// This is used for high-load tests to ensure that other tests do not timeout.
func PickCockroachAlt(t TB) string {
	if *cockroachAlt == "" {
		return PickCockroach(t)
	}
	if strings.EqualFold(*cockroachAlt, "omit") {
		t.Skip("Cockroach alt flag omitted.")
	}

	return pickNext(*cockroachAlt, &pickCockroach)
}

var pickPostgres atomic.Uint64
var pickCockroach atomic.Uint64
var pickTiDB atomic.Uint64

func pickNext(dbstr string, counter *atomic.Uint64) string {
	values := strings.Split(dbstr, "|")
	if len(values) <= 1 {
		return dbstr
	}
	v := counter.Add(1)
	return values[v%uint64(len(values))]
}
