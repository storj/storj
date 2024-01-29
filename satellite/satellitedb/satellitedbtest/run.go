// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

// This package should be referenced only in test files!

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/dbutil"
	"storj.io/common/dbutil/pgtest"
	"storj.io/common/dbutil/pgutil"
	"storj.io/common/dbutil/tempdb"
	"storj.io/common/tagsql"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

// Cockroach DROP DATABASE takes a significant amount, however, it has no importance in our tests.
var cockroachNoDrop = flag.Bool("cockroach-no-drop", stringToBool(os.Getenv("STORJ_TEST_COCKROACH_NODROP")), "Skip dropping cockroach databases to speed up tests")

func stringToBool(v string) bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

// SatelliteDatabases maybe name can be better.
type SatelliteDatabases struct {
	Name       string
	MasterDB   Database
	MetabaseDB Database
}

// Database describes a test database.
type Database struct {
	Name    string
	URL     string
	Message string
}

type ignoreSkip struct{}

func (ignoreSkip) Skip(...interface{}) {}

// Databases returns default databases.
func Databases() []SatelliteDatabases {
	var dbs []SatelliteDatabases

	postgresConnStr := pgtest.PickPostgres(ignoreSkip{})
	if !strings.EqualFold(postgresConnStr, "omit") {
		dbs = append(dbs, SatelliteDatabases{
			Name:       "Postgres",
			MasterDB:   Database{"Postgres", postgresConnStr, "Postgres flag missing, example: -postgres-test-db=" + pgtest.DefaultPostgres + " or use STORJ_TEST_POSTGRES environment variable."},
			MetabaseDB: Database{"Postgres", postgresConnStr, ""},
		})
	}

	cockroachConnStr := pgtest.PickCockroach(ignoreSkip{})
	if !strings.EqualFold(cockroachConnStr, "omit") {
		dbs = append(dbs, SatelliteDatabases{
			Name:       "Cockroach",
			MasterDB:   Database{"Cockroach", cockroachConnStr, "Cockroach flag missing, example: -cockroach-test-db=" + pgtest.DefaultCockroach + " or use STORJ_TEST_COCKROACH environment variable."},
			MetabaseDB: Database{"Cockroach", cockroachConnStr, ""},
		})
	}

	return dbs
}

// SchemaSuffix returns a suffix for schemas.
func SchemaSuffix() string {
	return pgutil.CreateRandomTestingSchemaName(6)
}

// SchemaName returns a properly formatted schema string.
func SchemaName(testname, category string, index int, schemaSuffix string) string {
	// The database is very lenient on allowed characters
	// but the same cannot be said for all tools
	nameCleaner := regexp.MustCompile(`[^\w]`)

	testname = nameCleaner.ReplaceAllString(testname, "_")
	category = nameCleaner.ReplaceAllString(category, "_")
	schemaSuffix = nameCleaner.ReplaceAllString(schemaSuffix, "_")

	// postgres has a maximum schema length of 64
	// we need additional 6 bytes for the random suffix
	//    and 4 bytes for the satellite index "/S0/""

	indexStr := strconv.Itoa(index)

	var maxTestNameLen = 64 - len(category) - len(indexStr) - len(schemaSuffix) - 2
	if len(testname) > maxTestNameLen {
		testname = testname[:maxTestNameLen]
	}

	if schemaSuffix == "" {
		return strings.ToLower(testname + "_" + category + indexStr)
	}

	return strings.ToLower(testname + "_" + schemaSuffix + "_" + category + indexStr)
}

// tempMasterDB is a satellite.DB-implementing type that cleans up after itself when closed.
type tempMasterDB struct {
	satellite.DB
	tempDB *dbutil.TempDatabase
}

// Close closes a tempMasterDB and cleans it up afterward.
func (db *tempMasterDB) Close() error {
	return errs.Combine(db.DB.Close(), db.tempDB.Close())
}

// CreateMasterDB creates a new satellite database for testing.
func CreateMasterDB(ctx context.Context, log *zap.Logger, name string, category string, index int, dbInfo Database, applicationName string) (db satellite.DB, err error) {
	if dbInfo.URL == "" {
		return nil, fmt.Errorf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
	}

	schemaSuffix := SchemaSuffix()
	log.Debug("creating", zap.String("suffix", schemaSuffix))
	schema := SchemaName(name, category, index, schemaSuffix)

	tempDB, err := tempdb.OpenUnique(ctx, dbInfo.URL, schema)
	if err != nil {
		return nil, err
	}
	if *cockroachNoDrop && tempDB.Driver == "cockroach" {
		tempDB.Cleanup = func(d tagsql.DB) error { return nil }
	}

	return CreateMasterDBOnTopOf(ctx, log, tempDB, applicationName)
}

// CreateMasterDBOnTopOf creates a new satellite database on top of an already existing
// temporary database.
func CreateMasterDBOnTopOf(ctx context.Context, log *zap.Logger, tempDB *dbutil.TempDatabase, applicationName string) (db satellite.DB, err error) {
	masterDB, err := satellitedb.Open(ctx, log.Named("db"), tempDB.ConnStr, satellitedb.Options{ApplicationName: applicationName})
	return &tempMasterDB{DB: masterDB, tempDB: tempDB}, err
}

// CreateMetabaseDB creates a new satellite metabase for testing.
func CreateMetabaseDB(ctx context.Context, log *zap.Logger, name string, category string, index int, dbInfo Database, config metabase.Config) (db *metabase.DB, err error) {
	if dbInfo.URL == "" {
		return nil, fmt.Errorf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
	}

	schemaSuffix := SchemaSuffix()
	log.Debug("creating", zap.String("suffix", schemaSuffix))

	schema := SchemaName(name, category, index, schemaSuffix)

	tempDB, err := tempdb.OpenUnique(ctx, dbInfo.URL, schema)
	if err != nil {
		return nil, err
	}
	if *cockroachNoDrop && tempDB.Driver == "cockroach" {
		tempDB.Cleanup = func(d tagsql.DB) error { return nil }
	}

	return CreateMetabaseDBOnTopOf(ctx, log, tempDB, config)
}

// CreateMetabaseDBOnTopOf creates a new metabase on top of an already existing
// temporary database.
func CreateMetabaseDBOnTopOf(ctx context.Context, log *zap.Logger, tempDB *dbutil.TempDatabase, config metabase.Config) (*metabase.DB, error) {
	db, err := metabase.Open(ctx, log.Named("metabase"), tempDB.ConnStr, config)
	if err != nil {
		return nil, err
	}
	db.TestingSetCleanup(tempDB.Close)
	return db, nil
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(ctx *testcontext.Context, t *testing.T, db satellite.DB)) {
	for _, dbInfo := range Databases() {
		dbInfo := dbInfo
		t.Run(dbInfo.Name, func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			if dbInfo.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", dbInfo.MasterDB.Name, dbInfo.MasterDB.Message)
			}

			logger := zaptest.NewLogger(t)
			applicationName := "satellite-satellitedb-test-" + pgutil.CreateRandomTestingSchemaName(6)
			db, err := CreateMasterDB(ctx, logger, t.Name(), "T", 0, dbInfo.MasterDB, applicationName)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				err := db.Close()
				if err != nil {
					t.Fatal(err)
				}
			}()

			err = db.Testing().TestMigrateToLatest(ctx)
			if err != nil {
				t.Fatal(err)
			}

			var fullScansBefore []string
			tempMasterDB, ok := db.(*tempMasterDB)
			if ok {
				fullScansBefore, err = FullTableScanQueries(ctx, tempMasterDB.tempDB.DB, tempMasterDB.tempDB.Implementation, applicationName)
				if err != nil {
					t.Fatal(err)
				}
			}

			test(ctx, t, db)

			if ok {
				fullScansAfter, err := FullTableScanQueries(ctx, tempMasterDB.tempDB.DB, tempMasterDB.tempDB.Implementation, applicationName)
				if err != nil {
					t.Fatal(err)
				}

				diff := cmp.Diff(fullScansBefore, fullScansAfter)
				if diff != "" {
					logger.Sugar().Warnf("FULL TABLE SCAN DETECTED\n%s", diff)
				}
			}
		})
	}
}

// Bench method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Bench(b *testing.B, bench func(b *testing.B, db satellite.DB)) {
	for _, dbInfo := range Databases() {
		dbInfo := dbInfo
		b.Run(dbInfo.Name, func(b *testing.B) {
			if dbInfo.MasterDB.URL == "" {
				b.Skipf("Database %s connection string not provided. %s", dbInfo.MasterDB.Name, dbInfo.MasterDB.Message)
			}

			ctx := testcontext.New(b)
			defer ctx.Cleanup()

			db, err := CreateMasterDB(ctx, zap.NewNop(), b.Name(), "X", 0, dbInfo.MasterDB, "satellite-satellitedb-bench")
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				err := db.Close()
				if err != nil {
					b.Fatal(err)
				}
			}()

			err = db.MigrateToLatest(ctx)
			if err != nil {
				b.Fatal(err)
			}

			// TODO: pass the ctx down
			bench(b, db)
		})
	}
}

// FullTableScanQueries is a helper method to list all queries which performed full table scan recently. It works only for cockroach db.
func FullTableScanQueries(ctx context.Context, db tagsql.DB, implementation dbutil.Implementation, applicationName string) (queries []string, err error) {
	if implementation != dbutil.Cockroach {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx,
		"SELECT key FROM crdb_internal.node_statement_statistics WHERE full_scan = TRUE AND application_name = $1 ORDER BY count DESC",
		applicationName,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	result := map[string]struct{}{}
	for rows.Next() {
		var query string
		err := rows.Scan(&query)
		if err != nil {
			return nil, err
		}

		// find smarter way to ignore known full table scan queries
		if !strings.Contains(strings.ToUpper(query), "WHERE") {
			continue
		}

		result[query] = struct{}{}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	for query := range result {
		queries = append(queries, query)
	}

	return queries, nil
}
