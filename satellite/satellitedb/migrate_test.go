// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/dbschema"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/dbutil/tempdb"
	"storj.io/storj/private/migrate"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// loadSnapshots loads all the dbschemas from testdata/postgres.*
func loadSnapshots(ctx context.Context, connstr, dbxscript string) (*dbschema.Snapshots, *dbschema.Schema, error) {
	snapshots := &dbschema.Snapshots{}

	// find all postgres sql files
	matches, err := filepath.Glob("testdata/postgres.*")
	if err != nil {
		return nil, nil, err
	}

	snapshots.List = make([]*dbschema.Snapshot, len(matches))
	var group errgroup.Group
	for i, match := range matches {
		i, match := i, match
		group.Go(func() error {
			versionStr := match[19 : len(match)-4] // hack to avoid trim issues with path differences in windows/linux
			version, err := strconv.Atoi(versionStr)
			if err != nil {
				return errs.New("invalid testdata file %q: %v", match, err)
			}

			scriptData, err := ioutil.ReadFile(match)
			if err != nil {
				return errs.New("could not read testdata file for version %d: %v", version, err)
			}

			snapshot, err := loadSnapshotFromSQL(ctx, connstr, string(scriptData))
			if err != nil {
				if pqErr, ok := err.(*pq.Error); ok && pqErr.Detail != "" {
					return fmt.Errorf("Version %d error: %v\nDetail: %s\nHint: %s", version, pqErr, pqErr.Detail, pqErr.Hint)
				}
				return fmt.Errorf("Version %d error: %+v", version, err)
			}
			snapshot.Version = version

			snapshots.List[i] = snapshot
			return nil
		})
	}
	var dbschema *dbschema.Schema
	group.Go(func() error {
		var err error
		dbschema, err = loadSchemaFromSQL(ctx, connstr, dbxscript)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, nil, err
	}

	snapshots.Sort()

	return snapshots, dbschema, nil
}

// loadSnapshotFromSQL inserts script into connstr and loads schema.
func loadSnapshotFromSQL(ctx context.Context, connstr, script string) (_ *dbschema.Snapshot, err error) {
	db, err := tempdb.OpenUnique(ctx, connstr, "load-schema")
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	_, err = db.ExecContext(ctx, script)
	if err != nil {
		return nil, err
	}

	snapshot, err := pgutil.QuerySnapshot(ctx, db)
	if err != nil {
		return nil, err
	}

	snapshot.Script = script
	return snapshot, nil
}

const newDataSeparator = `-- NEW DATA --`

func newData(snap *dbschema.Snapshot) string {
	tokens := strings.SplitN(snap.Script, newDataSeparator, 2)
	if len(tokens) != 2 {
		return ""
	}
	return tokens[1]
}

// loadSchemaFromSQL inserts script into connstr and loads schema.
func loadSchemaFromSQL(ctx context.Context, connstr, script string) (_ *dbschema.Schema, err error) {
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

func TestMigratePostgres(t *testing.T)  { migrateTest(t, pgtest.PickPostgres(t)) }
func TestMigrateCockroach(t *testing.T) { migrateTest(t, pgtest.PickCockroach(t)) }

// satelliteDB provides access to certain methods on a *satellitedb.satelliteDB
// instance, since that type is not exported.
type satelliteDB interface {
	TestDBAccess() *dbx.DB
	PostgresMigration() *migrate.Migration
}

func migrateTest(t *testing.T, connStr string) {
	t.Parallel()

	ctx := testcontext.NewWithTimeout(t, 8*time.Minute)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	// create tempDB
	tempDB, err := tempdb.OpenUnique(ctx, connStr, "migrate")
	require.NoError(t, err)
	defer func() { require.NoError(t, tempDB.Close()) }()

	// create a new satellitedb connection
	db, err := satellitedb.New(log, tempDB.ConnStr, satellitedb.Options{})
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	// we need raw database access unfortunately
	rawdb := db.(satelliteDB).TestDBAccess()

	snapshots, dbxschema, err := loadSnapshots(ctx, connStr, rawdb.Schema())
	require.NoError(t, err)

	var finalSchema *dbschema.Schema

	// get migration for this database
	migrations := db.(satelliteDB).PostgresMigration()
	for i, step := range migrations.Steps {
		tag := fmt.Sprintf("#%d - v%d", i, step.Version)

		// run migration up to a specific version
		err := migrations.TargetVersion(step.Version).Run(ctx, log.Named("migrate"))
		require.NoError(t, err, tag)

		// find the matching expected version
		expected, ok := snapshots.FindVersion(step.Version)
		require.True(t, ok, "Missing snapshot v%d. Did you forget to add a snapshot for the new migration?", step.Version)

		// insert data for new tables
		if newdata := newData(expected); newdata != "" {
			_, err = rawdb.ExecContext(ctx, newdata)
			require.NoError(t, err, tag)
		}

		// load schema from database
		currentSchema, err := pgutil.QuerySchema(ctx, rawdb)
		require.NoError(t, err, tag)

		// we don't care changes in versions table
		currentSchema.DropTable("versions")

		// load data from database
		currentData, err := pgutil.QueryData(ctx, rawdb, currentSchema)
		require.NoError(t, err, tag)

		// verify schema and data
		require.Equal(t, expected.Schema, currentSchema, tag)
		require.Equal(t, expected.Data, currentData, tag)

		// keep the last version around
		finalSchema = currentSchema
	}

	// verify that we also match the dbx version
	require.Equal(t, dbxschema, finalSchema, "result of all migration scripts did not match dbx schema")
}

func BenchmarkSetup_Postgres(b *testing.B) {
	connstr := pgtest.PickPostgres(b)
	b.Run("merged", func(b *testing.B) {
		benchmarkSetup(b, connstr, true)
	})
	b.Run("separate", func(b *testing.B) {
		benchmarkSetup(b, connstr, false)
	})
}

func BenchmarkSetup_Cockroach(b *testing.B) {
	connstr := pgtest.PickCockroach(b)
	b.Run("merged", func(b *testing.B) {
		benchmarkSetup(b, connstr, true)
	})
	b.Run("separate", func(b *testing.B) {
		benchmarkSetup(b, connstr, false)
	})
}

func benchmarkSetup(b *testing.B, connStr string, merged bool) {
	for i := 0; i < b.N; i++ {
		func() {
			ctx := context.Background()
			log := zap.NewNop()

			// create tempDB
			tempDB, err := tempdb.OpenUnique(ctx, connStr, "migrate")
			require.NoError(b, err)
			defer func() { require.NoError(b, tempDB.Close()) }()

			// create a new satellitedb connection
			db, err := satellitedb.New(log, tempDB.ConnStr, satellitedb.Options{})
			require.NoError(b, err)
			defer func() { require.NoError(b, db.Close()) }()

			if merged {
				err = db.TestingMigrateToLatest(ctx)
				require.NoError(b, err)
			} else {
				err = db.MigrateToLatest(ctx)
				require.NoError(b, err)
			}
		}()
	}
}
