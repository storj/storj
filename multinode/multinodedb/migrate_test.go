// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedb_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/dbutil/dbschema"
	"storj.io/common/dbutil/pgtest"
	"storj.io/common/dbutil/pgutil"
	"storj.io/common/dbutil/sqliteutil"
	"storj.io/common/dbutil/tempdb"
	"storj.io/common/testcontext"
	"storj.io/storj/multinode/multinodedb"
)

func TestMigrateSQLite3(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 8*time.Minute)
	defer ctx.Cleanup()
	log := zaptest.NewLogger(t)

	dbURL := "sqlite3://file::memory:"

	db, err := multinodedb.Open(ctx, log, dbURL)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	// get snapshots
	// find all sqlite3 sql files
	matches, err := filepath.Glob("testdata/sqlite3.*")
	require.NoError(t, err)
	snapshots := new(dbschema.Snapshots)
	snapshots.List = make([]*dbschema.Snapshot, len(matches))

	for i, match := range matches {
		version := parseTestdataVersion(match, "sqlite3")
		require.True(t, version >= 0, "invalid testdata file %q: %v", match, err)

		scriptData, err := os.ReadFile(match)
		require.NoError(t, err, "could not read testdata file for version %d: %v", version, err)

		// exec per snapshot??
		snapshot, err := sqliteutil.LoadSnapshotFromSQL(ctx, string(scriptData))
		require.NoError(t, err)
		snapshot.Version = version
		snapshots.List[i] = snapshot
	}

	snapshots.Sort()

	// get latest schema
	schema, err := sqliteutil.LoadSchemaFromSQL(ctx, db.Schema())
	require.NoError(t, err)

	var finalSchema *dbschema.Schema

	migration := db.SQLite3Migration()
	for i, step := range migration.Steps {
		tag := fmt.Sprintf("#%d - v%d", i, step.Version)

		expected, ok := snapshots.FindVersion(step.Version)
		require.True(t, ok)

		err = migration.TargetVersion(step.Version).Run(ctx, log)
		require.NoError(t, err)

		if newData := expected.LookupSection(dbschema.NewData); newData != "" {
			_, err = db.ExecContext(ctx, newData)
			require.NoError(t, err)
		}

		currentSchema, err := sqliteutil.QuerySchema(ctx, db)
		require.NoError(t, err)
		currentSchema.DropTable("versions")

		currentData, err := sqliteutil.QueryData(ctx, db, currentSchema)
		require.NoError(t, err)

		require.Equal(t, expected.Schema, currentSchema, tag)
		require.Equal(t, expected.Data, currentData, tag)

		finalSchema = currentSchema
	}

	// verify that we also match the dbx version
	require.Equal(t, schema, finalSchema, "result of all migration scripts did not match dbx schema")
}

func TestMigratePostgres(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 8*time.Minute)
	defer ctx.Cleanup()
	log := zaptest.NewLogger(t)

	connStr := pgtest.PickPostgres(t)

	// create tempDB
	tempDB, err := tempdb.OpenUnique(ctx, connStr, "migrate")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tempDB.Close())
	}()

	db, err := multinodedb.Open(ctx, log, tempDB.ConnStr)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	// get snapshots
	// find all postgres sql files
	matches, err := filepath.Glob("testdata/postgres.*")
	require.NoError(t, err)
	snapshots := new(dbschema.Snapshots)
	snapshots.List = make([]*dbschema.Snapshot, len(matches))

	for i, match := range matches {
		version := parseTestdataVersion(match, "postgres")
		require.True(t, version >= 0, "invalid testdata file %q: %v", match, err)

		scriptData, err := os.ReadFile(match)
		require.NoError(t, err, "could not read testdata file for version %d: %v", version, err)

		snapshot, err := loadSnapshotFromSQLPostgres(ctx, connStr, string(scriptData))
		require.NoError(t, err)
		snapshot.Version = version
		snapshots.List[i] = snapshot
	}

	snapshots.Sort()

	// get latest schema
	schema, err := loadSchemaFromSQLPostgres(ctx, connStr, db.Schema())
	require.NoError(t, err)

	var finalSchema *dbschema.Schema

	migration := db.PostgresMigration()
	for i, step := range migration.Steps {
		tag := fmt.Sprintf("#%d - v%d", i, step.Version)

		expected, ok := snapshots.FindVersion(step.Version)
		require.True(t, ok)

		err = migration.TargetVersion(step.Version).Run(ctx, log)
		require.NoError(t, err)

		if newData := expected.LookupSection(dbschema.NewData); newData != "" {
			_, err = db.ExecContext(ctx, newData)
			require.NoError(t, err)
		}

		currentSchema, err := pgutil.QuerySchema(ctx, db)
		require.NoError(t, err)
		currentSchema.DropTable("versions")

		currentData, err := pgutil.QueryData(ctx, db, currentSchema)
		require.NoError(t, err)

		require.Equal(t, expected.Schema, currentSchema, tag)
		require.Equal(t, expected.Data, currentData, tag)

		finalSchema = currentSchema
	}

	// verify that we also match the dbx version
	require.Equal(t, schema, finalSchema, "result of all migration scripts did not match dbx schema")
}

func parseTestdataVersion(path string, impl string) int {
	path = filepath.ToSlash(strings.ToLower(path))
	path = strings.TrimPrefix(path, "testdata/"+impl+".v")
	path = strings.TrimSuffix(path, ".sql")

	v, err := strconv.Atoi(path)
	if err != nil {
		return -1
	}
	return v
}

// loadSnapshotFromSQLPostgres inserts script into connstr and loads snapshot for postgres db.
func loadSnapshotFromSQLPostgres(ctx context.Context, connstr, script string) (_ *dbschema.Snapshot, err error) {
	db, err := tempdb.OpenUnique(ctx, connstr, "load-schema")
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	sections := dbschema.NewSections(script)

	_, err = db.ExecContext(ctx, sections.LookupSection(dbschema.Main))
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, sections.LookupSection(dbschema.MainData))
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, sections.LookupSection(dbschema.NewData))
	if err != nil {
		return nil, err
	}

	snapshot, err := pgutil.QuerySnapshot(ctx, db)
	if err != nil {
		return nil, err
	}
	snapshot.Sections = sections
	return snapshot, nil
}

// loadSnapshotFromSQLPostgres inserts script into connstr and loads schema for postgres db.
func loadSchemaFromSQLPostgres(ctx context.Context, connstr, script string) (_ *dbschema.Schema, err error) {
	db, err := tempdb.OpenUnique(ctx, connstr, "load-schema")
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	_, err = db.ExecContext(ctx, script)
	if err != nil {
		return nil, err
	}

	return pgutil.QuerySchema(ctx, db)
}
