// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedbtest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/multinodedb"
	"storj.io/storj/multinode/multinodedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/tempdb"
)

// Database describes a test database.
type Database struct {
	Name    string
	URL     string
	Message string
}

// tempMasterDB is a multinode.DB-implementing type that cleans up after itself when closed.
type tempMasterDB struct {
	multinode.DB
	tempDB *dbutil.TempDatabase
}

// Close closes a tempMasterDB and cleans it up afterward.
func (db *tempMasterDB) Close() error {
	return errs.Combine(db.DB.Close(), db.tempDB.Close())
}

// TestDBAccess provides a somewhat regularized access to the underlying DB.
func (db *tempMasterDB) TestDBAccess() *dbx.DB {
	return db.DB.(interface{ TestDBAccess() *dbx.DB }).TestDBAccess()
}

// SchemaSuffix returns a suffix for schemas.
func SchemaSuffix() string {
	return pgutil.CreateRandomTestingSchemaName(6)
}

// SchemaName returns a properly formatted schema string.
func SchemaName(testname, category string, index int, schemaSuffix string) string {
	// postgres has a maximum schema length of 64
	// we need additional 6 bytes for the random suffix
	// and 4 bytes for the index "/S0/""

	indexStr := strconv.Itoa(index)

	maxTestNameLen := 64 - len(category) - len(indexStr) - len(schemaSuffix) - 2
	if len(testname) > maxTestNameLen {
		testname = testname[:maxTestNameLen]
	}

	if schemaSuffix == "" {
		return strings.ToLower(testname + "/" + category + indexStr)
	}

	return strings.ToLower(testname + "/" + schemaSuffix + "/" + category + indexStr)
}

// CreateMasterDB creates a new satellite database for testing.
func CreateMasterDB(ctx context.Context, log *zap.Logger, name string, category string, index int, dbInfo Database) (db multinode.DB, err error) {
	if dbInfo.URL == "" {
		return nil, fmt.Errorf("database %s connection string not provided. %s", dbInfo.Name, dbInfo.Message)
	}

	schemaSuffix := SchemaSuffix()
	log.Debug("creating", zap.String("suffix", schemaSuffix))
	schema := SchemaName(name, category, index, schemaSuffix)

	tempDB, err := tempdb.OpenUnique(ctx, log, dbInfo.URL, schema, nil)
	if err != nil {
		return nil, err
	}

	return CreateMasterDBOnTopOf(ctx, log, tempDB)
}

// CreateMasterDBOnTopOf creates a new satellite database on top of an already existing
// temporary database.
func CreateMasterDBOnTopOf(ctx context.Context, log *zap.Logger, tempDB *dbutil.TempDatabase) (db multinode.DB, err error) {
	masterDB, err := multinodedb.Open(ctx, log, tempDB.ConnStr)
	return &tempMasterDB{DB: masterDB, tempDB: tempDB}, err
}

// Run method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func Run(t *testing.T, test func(ctx *testcontext.Context, t *testing.T, db multinode.DB)) {
	t.Parallel()

	databases := []Database{
		{
			Name:    "Postgres",
			URL:     dbtest.PickPostgresNoSkip(),
			Message: "Postgres flag missing, example: -postgres-test-db=" + dbtest.DefaultPostgres + " or use STORJ_TEST_POSTGRES environment variable.",
		},
		{
			Name: "Sqlite3",
			URL:  "sqlite3://file::memory:",
		},
	}

	for _, database := range databases {
		dbConfig := database

		t.Run(dbConfig.Name, func(t *testing.T) {
			t.Parallel()

			if dbConfig.URL == "" || dbConfig.URL == "omit" {
				t.Skipf("Database %s connection string not provided. %s", dbConfig.Name, dbConfig.Message)
			}

			log := zaptest.NewLogger(t)
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			var db multinode.DB
			var err error

			if dbConfig.Name == "Postgres" {
				db, err = CreateMasterDB(ctx, log, t.Name(), "T", 0, dbConfig)
			} else {
				db, err = multinodedb.Open(ctx, log, dbConfig.URL)
			}
			if err != nil {
				t.Fatal(err)
			}

			defer ctx.Check(db.Close)

			err = db.MigrateToLatest(ctx)
			if err != nil {
				t.Fatal(err)
			}

			test(ctx, t, db)
		})
	}
}
