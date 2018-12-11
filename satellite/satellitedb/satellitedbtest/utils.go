// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

// This package should be referenced only in test files!

import (
	"flag"
	"os"
	"testing"

	"storj.io/storj/satellite/satellitedb"
)

const (
	// postgres connstring that works with docker-compose
	defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
	defaultSqliteConn   = "sqlite3://file::memory:?mode=memory&cache=shared"
)

var (
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db *satellitedb.DB)) {
	for _, dbInfo := range []struct {
		dbName    string
		dbURL     string
		dbMessage string
	}{
		{"Sqlite", defaultSqliteConn, ""},
		//{"Postgres", *testPostgres, "Postgres flag missing, example: -postgres-test-db=" + defaultPostgresConn},
	} {
		t.Run(dbInfo.dbName, func(t *testing.T) {
			if dbInfo.dbURL == "" {
				t.Skipf("Database %s connection string not provided. %s", dbInfo.dbName, dbInfo.dbMessage)
			}

			db, err := satellitedb.New(dbInfo.dbURL)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := db.Close()
				if err != nil {
					t.Fatal(err)
				}
			}()

			err = db.CreateTables()
			if err != nil {
				t.Fatal(err)
			}

			test(t, db)
		})
	}
}
