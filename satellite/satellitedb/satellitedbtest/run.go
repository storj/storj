// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

// This package should be referenced only in test files!

import (
	"strings"
	"testing"

	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/dbutil/pgutil"
	"storj.io/storj/internal/dbutil/pgutil/pgtest"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
)

const (
	// DefaultSqliteConn is a connstring that is inmemory
	DefaultSqliteConn = "sqlite3://file::memory:?mode=memory"
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
		{"Postgres", *pgtest.ConnStr, "Postgres flag missing, example: -postgres-test-db=" + pgtest.DefaultConnStr},
	}
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db satellite.DB)) {
	schemaSuffix := pgutil.CreateRandomTestingSchemaName(8)
	t.Log("schema-suffix ", schemaSuffix)

	for _, dbInfo := range Databases() {
		dbInfo := dbInfo
		t.Run(dbInfo.Name, func(t *testing.T) {
			t.Parallel()

			if dbInfo.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
			}

			log := zaptest.NewLogger(t)

			schema := strings.ToLower(t.Name() + "-satellite/x-" + schemaSuffix)
			connstr := pgutil.ConnstrWithSchema(dbInfo.URL, schema)
			db, err := satellitedb.New(log, connstr)
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
