// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

// This package should be referenced only in test files!

import (
	"strings"
	"testing"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
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

// SatelliteDatabases maybe name can be better
type SatelliteDatabases struct {
	MasterDB  Database
	PointerDB Database
}

// Database describes a test database
type Database struct {
	Name    string
	URL     string
	Message string
}

// Databases returns default databases.
func Databases() []SatelliteDatabases {
	return []SatelliteDatabases{
		{
			MasterDB:  Database{"Sqlite", DefaultSqliteConn, ""},
			PointerDB: Database{"Bolt", "", "should use preconfigured URL"},
		},
		{
			MasterDB:  Database{"Postgres", *pgtest.ConnStr, "Postgres flag missing, example: -postgres-test-db=" + pgtest.DefaultConnStr},
			PointerDB: Database{"Postgres", *pgtest.ConnStr, ""},
		},
	}
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db satellite.DB)) {
	schemaSuffix := pgutil.CreateRandomTestingSchemaName(8)
	t.Log("schema-suffix ", schemaSuffix)

	for _, dbInfo := range Databases() {
		dbInfo := dbInfo
		t.Run(dbInfo.MasterDB.Name+"/"+dbInfo.PointerDB.Name, func(t *testing.T) {
			t.Parallel()

			if dbInfo.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", dbInfo.MasterDB.Name, dbInfo.MasterDB.Message)
			}

			log := zaptest.NewLogger(t)

			schema := strings.ToLower(t.Name() + "-satellite/x-" + schemaSuffix)
			connstr := pgutil.ConnstrWithSchema(dbInfo.MasterDB.URL, schema)
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

// Bench method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Bench(b *testing.B, bench func(b *testing.B, db satellite.DB)) {
	schemaSuffix := pgutil.CreateRandomTestingSchemaName(8)
	b.Log("schema-suffix ", schemaSuffix)

	for _, dbInfo := range Databases() {
		dbInfo := dbInfo
		b.Run(dbInfo.MasterDB.Name+"/"+dbInfo.PointerDB.Name, func(b *testing.B) {
			if dbInfo.MasterDB.URL == "" {
				b.Skipf("Database %s connection string not provided. %s", dbInfo.MasterDB.Name, dbInfo.MasterDB.Message)
			}

			log := zap.NewNop()

			schema := strings.ToLower(b.Name() + "-satellite/x-" + schemaSuffix)
			connstr := pgutil.ConnstrWithSchema(dbInfo.MasterDB.URL, schema)
			db, err := satellitedb.New(log, connstr)
			if err != nil {
				b.Fatal(err)
			}

			err = db.CreateSchema(schema)
			if err != nil {
				b.Fatal(err)
			}

			defer func() {
				dropErr := db.DropSchema(schema)
				err := errs.Combine(dropErr, db.Close())
				if err != nil {
					b.Fatal(err)
				}
			}()

			err = db.CreateTables()
			if err != nil {
				b.Fatal(err)
			}

			bench(b, db)
		})
	}
}
