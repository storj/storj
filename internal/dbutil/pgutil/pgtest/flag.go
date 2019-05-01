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

// DefaultConnStr is expected to work under the storj-test docker-compose instance
const DefaultConnStr = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
