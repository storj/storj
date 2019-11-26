// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

// This package should be referenced only in test files!

import (
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
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
			MasterDB:  Database{"Postgres", *pgtest.ConnStr, "Postgres flag missing, example: -postgres-test-db=" + pgtest.DefaultConnStr + " or use STORJ_POSTGRES_TEST environment variable."},
			PointerDB: Database{"Postgres", *pgtest.ConnStr, ""},
		},
		{
			MasterDB:  Database{"Cockroach", *pgtest.CrdbConnStr, "Cockroach flag missing, example: -cockroach-test-db=" + pgtest.DefaultCrdbConnStr + " or use STORJ_COCKROACH_TEST environment variable."},
			PointerDB: Database{"Postgres", *pgtest.ConnStr, ""},
		},
	}
}

// SchemaSuffix returns a suffix for schemas.
func SchemaSuffix() string {
	return pgutil.CreateRandomTestingSchemaName(6)
}

// SchemaName returns a properly formatted schema string.
func SchemaName(testname, category string, index int, schemaSuffix string) string {
	// postgres has a maximum schema length of 64
	// we need additional 6 bytes for the random suffix
	//    and 4 bytes for the satellite index "/S0/""

	indexStr := strconv.Itoa(index)

	var maxTestNameLen = 64 - len(category) - len(indexStr) - len(schemaSuffix) - 2
	if len(testname) > maxTestNameLen {
		testname = testname[:maxTestNameLen]
	}

	if schemaSuffix == "" {
		return strings.ToLower(testname + "/" + category + indexStr)
	}

	return strings.ToLower(testname + "/" + schemaSuffix + "/" + category + indexStr)
}

// CreateMasterDB creates a new satellite database for testing
func CreateMasterDB(t *testing.T, category string, index int, dbInfo Database) (db satellite.DB, err error) {
	if dbInfo.URL == "" {
		t.Fatalf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
	}

	schemaSuffix := SchemaSuffix()
	t.Log("schema-suffix ", schemaSuffix)

	log := zaptest.NewLogger(t)
	schema := SchemaName(t.Name(), category, index, schemaSuffix)

	switch dbInfo.Name {
	case "Postgres":
		db, err = NewPostgres(log.Named("db"), schema)
	case "Cockroach":
		db, err = NewCockroach(log.Named("db"), schema)
	default:
		db, err = satellitedb.New(log.Named("db"), dbInfo.URL)
	}
	return db, err
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(t *testing.T, db satellite.DB)) {
	for _, dbInfo := range Databases() {
		dbInfo := dbInfo
		t.Run(dbInfo.MasterDB.Name+"/"+dbInfo.PointerDB.Name, func(t *testing.T) {
			t.Parallel()

			// TODO: remove this skip once all the sql is cockroachdb compatible
			if dbInfo.MasterDB.Name == "Cockroach" {
				t.Skip("CockroachDB not supported yet")
			}

			db, err := CreateMasterDB(t, "T", 0, dbInfo.MasterDB)
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

// Bench method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Bench(b *testing.B, bench func(b *testing.B, db satellite.DB)) {
	for _, dbInfo := range Databases() {
		dbInfo := dbInfo
		b.Run(dbInfo.MasterDB.Name+"/"+dbInfo.PointerDB.Name, func(b *testing.B) {
			if dbInfo.MasterDB.URL == "" {
				b.Skipf("Database %s connection string not provided. %s", dbInfo.MasterDB.Name, dbInfo.MasterDB.Message)
			}

			schemaSuffix := SchemaSuffix()
			b.Log("schema-suffix ", schemaSuffix)

			log := zaptest.NewLogger(b)
			schema := SchemaName(b.Name(), "X", 0, schemaSuffix)

			db, err := satellitedb.New(log, dbInfo.MasterDB.URL)
			if err != nil {
				b.Fatal(err)
			}

			if dbInfo.MasterDB.Name == "Postgres" {
				pgdb, err := satellitedb.New(log, pgutil.ConnstrWithSchema(dbInfo.MasterDB.URL, schema))
				if err != nil {
					b.Fatal(err)
				}

				db = &SchemaDB{
					DB:       pgdb,
					Schema:   schema,
					AutoDrop: true,
				}
			}

			defer func() {
				err := db.Close()
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
