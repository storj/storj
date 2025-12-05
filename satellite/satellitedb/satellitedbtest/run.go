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
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/tempdb"
	"storj.io/storj/shared/tagsql"
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

	ExtraStatements []string // TODO: only implemented for spanner at the moment.
}

// Databases returns default databases.
func Databases[T dbtest.TB](t T) []SatelliteDatabases {
	var dbs []SatelliteDatabases

	postgresConnStr := dbtest.PickPostgresNoSkip()
	if !strings.EqualFold(postgresConnStr, "omit") {
		dbs = append(dbs, SatelliteDatabases{
			Name:       "Postgres",
			MasterDB:   Database{"Postgres", postgresConnStr, "Postgres flag missing, example: -postgres-test-db=" + dbtest.DefaultPostgres + " or use STORJ_TEST_POSTGRES environment variable.", nil},
			MetabaseDB: Database{"Postgres", postgresConnStr, "", nil},
		})
	}

	cockroachConnStr := dbtest.PickCockroachNoSkip()
	if !strings.EqualFold(cockroachConnStr, "omit") {
		dbs = append(dbs, SatelliteDatabases{
			Name:       "Cockroach",
			MasterDB:   Database{"Cockroach", cockroachConnStr, "Cockroach flag missing, example: -cockroach-test-db=" + dbtest.DefaultCockroach + " or use STORJ_TEST_COCKROACH environment variable.", nil},
			MetabaseDB: Database{"Cockroach", cockroachConnStr, "", nil},
		})
	}

	spanner := dbtest.PickSpannerNoSkip()
	if !strings.EqualFold(spanner, "omit") {
		// PickSpanner may start a server.
		connstr := dbtest.PickOrStartSpanner(t)
		dbs = append(dbs, SatelliteDatabases{
			Name:       "Spanner",
			MasterDB:   Database{"Spanner", connstr, "Spanner flag missing, example: -spanner-test-db=" + dbtest.DefaultSpanner + " or use STORJ_TEST_SPANNER environment variable.", satellitedb.SpannerExtraStatements},
			MetabaseDB: Database{"Spanner", connstr, "", nil},
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

	// spanner has a maximum database length of 30 while postgres has a maximum schema length of 64
	// we need additional 6 bytes for the random suffix and 4 bytes for the satellite index "/S0/""
	// additionally, we will leave 5 bytes for a delimiter and any randomness that need to be added for testing or
	// other purposes

	indexStr := strconv.Itoa(index)

	maxTestNameLen := 30 - len(category) - len(indexStr) - len(schemaSuffix) - 2 - 5
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
func CreateMasterDB(ctx context.Context, log *zap.Logger, name string, category string, index int, dbInfo Database, options satellitedb.Options) (db satellite.DB, err error) {
	if dbInfo.URL == "" {
		return nil, fmt.Errorf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
	}

	schemaSuffix := SchemaSuffix()
	log.Debug("creating", zap.String("suffix", schemaSuffix))
	schema := SchemaName(name, category, index, schemaSuffix)

	extraStatements := slices.Clone(dbInfo.ExtraStatements)
	tempDB, err := tempdb.OpenUnique(ctx, log, dbInfo.URL, schema, extraStatements)
	if err != nil {
		return nil, err
	}
	if *cockroachNoDrop && tempDB.Driver == "cockroach" {
		tempDB.Cleanup = func(d tagsql.DB) error { return nil }
	}

	return CreateMasterDBOnTopOf(ctx, log, tempDB, options)
}

// CreateMasterDBOnTopOf creates a new satellite database on top of an already existing
// temporary database.
func CreateMasterDBOnTopOf(ctx context.Context, log *zap.Logger, tempDB *dbutil.TempDatabase, options satellitedb.Options) (db satellite.DB, err error) {
	masterDB, err := satellitedb.Open(ctx, log.Named("db"), tempDB.ConnStr, options)
	return &tempMasterDB{DB: masterDB, tempDB: tempDB}, err
}

// TempDBSchemaConfig defines parameters required for the temp database.
type TempDBSchemaConfig struct {
	Name     string
	Category string
	Index    int
}

// CreateTempDB creates a new temporary database (Cockroach or Postgresql).
func CreateTempDB(ctx context.Context, log *zap.Logger, tcfg TempDBSchemaConfig, dbInfo Database) (db *dbutil.TempDatabase, err error) {
	if dbInfo.URL == "" {
		return nil, fmt.Errorf("Database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
	}

	schemaSuffix := SchemaSuffix()
	log.Debug("creating", zap.String("suffix", schemaSuffix))

	schema := SchemaName(tcfg.Name, tcfg.Category, tcfg.Index, schemaSuffix)

	tempDB, err := tempdb.OpenUnique(ctx, log, dbInfo.URL, schema, dbInfo.ExtraStatements)
	if err != nil {
		return nil, err
	}
	if *cockroachNoDrop && tempDB.Driver == "cockroach" {
		tempDB.Cleanup = func(d tagsql.DB) error { return nil }
	}

	return tempDB, nil
}

// CreateMetabaseDB creates a new satellite metabase for testing.
func CreateMetabaseDB(ctx context.Context, log *zap.Logger, name string, category string, index int, dbInfo Database, config metabase.Config) (db *metabase.DB, err error) {
	tempDB, err := CreateTempDB(ctx, log, TempDBSchemaConfig{
		Name:     name,
		Category: category,
		Index:    index,
	}, dbInfo)
	if err != nil {
		return nil, err
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
	RunWithConfig(t, Config{}, test)
}

// Config allows customizing Run behaviour.
type Config struct {
	NonParallel bool
}

// RunWithConfig method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func RunWithConfig(t *testing.T, cfg Config, test func(ctx *testcontext.Context, t *testing.T, db satellite.DB)) {
	if !cfg.NonParallel {
		t.Parallel()
	}
	for _, dbInfo := range Databases(t) {
		dbInfo := dbInfo
		t.Run(dbInfo.Name, func(t *testing.T) {
			if !cfg.NonParallel {
				t.Parallel()
			}

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			if dbInfo.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", dbInfo.MasterDB.Name, dbInfo.MasterDB.Message)
			}

			logger := zaptest.NewLogger(t)
			applicationName := "satellite-satellitedb-test-" + pgutil.CreateRandomTestingSchemaName(6)

			db, err := CreateMasterDB(ctx, logger, t.Name(), "T", 0, dbInfo.MasterDB, satellitedb.Options{
				ApplicationName: applicationName,
			})
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

			test(ctx, t, db)
		})
	}
}

// Bench method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Bench(b *testing.B, bench func(ctx *testcontext.Context, b *testing.B, db satellite.DB)) {
	for _, dbInfo := range Databases(b) {
		dbInfo := dbInfo
		b.Run(dbInfo.Name, func(b *testing.B) {
			if dbInfo.MasterDB.URL == "" {
				b.Skipf("Database %s connection string not provided. %s", dbInfo.MasterDB.Name, dbInfo.MasterDB.Message)
			}

			ctx := testcontext.NewWithTimeout(b, 30*time.Minute)
			defer ctx.Cleanup()

			db, err := CreateMasterDB(ctx, zap.NewNop(), b.Name(), "X", 0, dbInfo.MasterDB, satellitedb.Options{
				ApplicationName: "satellite-satellitedb-bench",
			})
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				err := db.Close()
				if err != nil {
					b.Fatal(err)
				}
			}()

			err = db.Testing().TestMigrateToLatest(ctx)
			if err != nil {
				b.Fatal(err)
			}

			bench(ctx, b, db)
		})
	}
}
