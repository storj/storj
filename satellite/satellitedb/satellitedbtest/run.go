// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

// This package should be referenced only in test files!

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"os"
	"testing"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
)

const (
	// DefaultPostgresConn is a connstring that works with docker-compose
	DefaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
	// DefaultSqliteConn is a connstring that is inmemory
	DefaultSqliteConn = "sqlite3://file::memory:?mode=memory"
)

var (
	// TestPostgres is flag for the postgres test database
	TestPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

// Database describes a test database
type Database struct {
	Name    string
	URL     string
	Message string
}

// Databases returns default databases.
func Databases() []Database {
	return []Database{
		{"Sqlite", DefaultSqliteConn, ""},
		{"Postgres", *TestPostgres, "Postgres flag missing, example: -postgres-test-db=" + DefaultPostgresConn},
	}
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db satellite.DB)) {
	schemaSuffix := randomSchemaSuffix()
	t.Log("schema-suffix ", schemaSuffix)

	for _, dbInfo := range Databases() {
		t.Run(dbInfo.Name, func(t *testing.T) {
			if dbInfo.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
			}

			db, err := satellitedb.New(dbInfo.URL)
			if err != nil {
				t.Fatal(err)
			}

			schema := t.Name() + "-satellite/x-" + schemaSuffix

			err = db.SetSchema(schema)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				dropErr := db.DropSchema(schema)
				err := errs.Combine(dropErr, db.Close())
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

func randomSchemaSuffix() string {
	var data [8]byte
	_, _ = rand.Read(data[:])
	return hex.EncodeToString(data[:])
}
