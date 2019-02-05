// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

// This package should be referenced only in test files!

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"net/url"
	"os"
	"strings"
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

// WithSchema adds schema param to connection string.
func WithSchema(connstring string, schema string) string {
	if strings.HasPrefix(connstring, "postgres") {
		return connstring + "&search_path=" + url.QueryEscape(schema)
	}
	return connstring
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db satellite.DB)) {
	schemaSuffix := randomSchemaSuffix()
	t.Log("schema-suffix ", schemaSuffix)

	for _, dbInfo := range Databases() {
		t.Run(dbInfo.Name, func(t *testing.T) {
			t.Parallel()

			if dbInfo.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
			}

			schema := strings.ToLower(t.Name() + "-satellite/x-" + schemaSuffix)
			db, err := satellitedb.New(WithSchema(dbInfo.URL, schema))
			if err != nil {
				t.Fatal(err)
			}

			err = db.CreateSchema(schema)
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
