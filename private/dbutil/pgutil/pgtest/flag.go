// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgtest

import (
	"flag"
	"os"
)

// We need to define this in a separate package due to https://golang.org/issue/23910.

// ConnStr is the test database connection string.
var ConnStr = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")

// CrdbConnStr is the test database connection string for CockroachDB
var CrdbConnStr = flag.String("cockroach-test-db", os.Getenv("STORJ_COCKROACH_TEST"), "CockroachDB test database connection string")

// DefaultConnStr is expected to work under the storj-test docker-compose instance
const DefaultConnStr = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"

// DefaultCrdbConnStr is expected to work when a local cockroachDB instance is running
const DefaultCrdbConnStr = "postgres://root@localhost:26257/master?sslmode=disable"
