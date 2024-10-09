// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/storj/private/migrate"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/tempdb"
)

const maxMigrationsToTest = 10

// loadSnapshots loads all the dbschemas from `testdata/postgres.*`.
func loadSnapshots(ctx context.Context, connstr string, schema []string, maxSnapshots int) (*dbschema.Snapshots, *dbschema.Schema, error) {
	snapshots := &dbschema.Snapshots{}

	dbxscript := strings.Join(schema, ";\n")
	// find all postgres sql files
	matches, err := filepath.Glob("testdata/postgres.*")
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(matches)

	// Limit the number of snapshots we are checking
	// because the database creation is not as fast.
	if len(matches) > maxSnapshots {
		matches = matches[len(matches)-maxSnapshots:]
	}

	snapshots.List = make([]*dbschema.Snapshot, len(matches))

	var sem sync2.Semaphore
	if strings.Contains(connstr, "cockroach") {
		sem.Init(4)
	} else {
		sem.Init(16)
	}

	var group errgroup.Group
	for i, match := range matches {
		i, match := i, match
		group.Go(func() error {
			sem.Lock()
			defer sem.Unlock()

			version := parseTestdataVersion(match)
			if version < 0 {
				return errs.New("invalid testdata file %q: %v", match, err)
			}

			scriptData, err := os.ReadFile(match)
			if err != nil {
				return errs.New("could not read testdata file for version %d: %v", version, err)
			}

			snapshot, err := loadSnapshotFromSQL(ctx, connstr, string(scriptData))
			if err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) {
					return fmt.Errorf("Version %d error: %w\nDetail: %s\nHint: %s", version, pgErr, pgErr.Detail, pgErr.Hint)
				}
				return fmt.Errorf("Version %d error: %w", version, err)
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

func parseTestdataVersion(path string) int {
	path = filepath.ToSlash(strings.ToLower(path))
	path = strings.TrimPrefix(path, "testdata/postgres.v")
	path = strings.TrimSuffix(path, ".sql")

	v, err := strconv.Atoi(path)
	if err != nil {
		return -1
	}
	return v
}

// loadSnapshotFromSQL inserts script into connstr and loads schema.
func loadSnapshotFromSQL(ctx context.Context, connstr, script string) (_ *dbschema.Snapshot, err error) {
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

func TestMigratePostgres(t *testing.T) {
	t.Parallel()
	connstr := dbtest.PickPostgres(t)
	t.Run("Versions", func(t *testing.T) { migrateTest(t, connstr) })
	t.Run("Generated", func(t *testing.T) { migrateGeneratedTest(t, connstr, connstr) })
}

func TestMigrateCockroach(t *testing.T) {
	t.Parallel()
	connstr := dbtest.PickCockroachAlt(t)
	t.Run("Versions", func(t *testing.T) { migrateTest(t, connstr) })
	t.Run("Generated", func(t *testing.T) { migrateGeneratedTest(t, connstr, connstr) })
}

func migrateTest(t *testing.T, connStr string) {
	ctx := testcontext.NewWithTimeout(t, 8*time.Minute)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	// create tempDB
	tempDB, err := tempdb.OpenUnique(ctx, connStr, "migrate")
	require.NoError(t, err)
	defer func() { require.NoError(t, tempDB.Close()) }()

	// create a new satellitedb connection
	db, err := satellitedb.Open(ctx, log, tempDB.ConnStr, satellitedb.Options{ApplicationName: "satellite-migration-test"})
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	// we need raw database access unfortunately
	rawdb := db.Testing().RawDB()

	loadingStart := time.Now()
	snapshots, dbxschema, err := loadSnapshots(ctx, connStr, db.Testing().Schema(), maxMigrationsToTest)
	require.NoError(t, err)
	t.Logf("snapshot loading %v", time.Since(loadingStart))

	// get migration for this database
	migrations := db.Testing().ProductionMigration()

	// find the first matching migration step for the snapshots
	firstSnapshot := snapshots.List[0]
	stepIndex := func() int {
		for i, step := range migrations.Steps {
			if step.Version == firstSnapshot.Version {
				return i
			}
		}
		return -1
	}()

	// migrate up to the first loaded snapshot
	err = migrations.TargetVersion(firstSnapshot.Version).Run(ctx, log.Named("initial-migration"))
	require.NoError(t, err)
	_, err = rawdb.ExecContext(ctx, firstSnapshot.LookupSection(dbschema.MainData))
	require.NoError(t, err)
	_, err = rawdb.ExecContext(ctx, firstSnapshot.LookupSection(dbschema.NewData))
	require.NoError(t, err)

	// test rest of the steps with snapshots
	var finalSchema *dbschema.Schema
	for i, step := range migrations.Steps[stepIndex+1:] {
		tag := fmt.Sprintf("#%d - v%d", i, step.Version)

		// find the matching expected version
		expected, ok := snapshots.FindVersion(step.Version)
		require.True(t, ok, "Missing snapshot v%d. Did you forget to add a snapshot for the new migration?", step.Version)

		// run any queries that should happen before the migration
		if oldData := expected.LookupSection(dbschema.OldData); oldData != "" {
			_, err = rawdb.ExecContext(ctx, oldData)
			require.NoError(t, err, tag)
		}

		// run migration up to a specific version
		err := migrations.TargetVersion(step.Version).Run(ctx, log.Named("migrate"))
		require.NoError(t, err, tag)

		// insert data for new tables
		if newData := expected.LookupSection(dbschema.NewData); newData != "" {
			_, err = rawdb.ExecContext(ctx, newData)
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

// migrateGeneratedTest verifies whether the generated code in `migratez.go` is on par with migrate.go.
func migrateGeneratedTest(t *testing.T, connStrProd, connStrTest string) {
	ctx := testcontext.NewWithTimeout(t, 8*time.Minute)
	defer ctx.Cleanup()

	prodVersion, prodSnapshot := schemaFromMigration(t, ctx, connStrProd, func(db satellite.DB) *migrate.Migration {
		return db.Testing().ProductionMigration()
	})

	testVersion, testSnapshot := schemaFromMigration(t, ctx, connStrTest, func(db satellite.DB) *migrate.Migration {
		return db.Testing().TestMigration()
	})

	assert.Equal(t, prodVersion, testVersion, "migratez version does not match migration. Run `go generate` to update.")

	prodSnapshot.DropTable("versions")
	testSnapshot.DropTable("versions")

	require.Equal(t, prodSnapshot.Schema, testSnapshot.Schema, "migratez schema does not match migration. Run `go generate` to update.")
	require.Equal(t, prodSnapshot.Data, testSnapshot.Data, "migratez data does not match migration. Run `go generate` to update.")
}

func schemaFromMigration(t *testing.T, ctx *testcontext.Context, connStr string, getMigration func(db satellite.DB) *migrate.Migration) (version int, _ *dbschema.Snapshot) {
	// create tempDB
	log := zaptest.NewLogger(t)

	tempDB, err := tempdb.OpenUnique(ctx, connStr, "migrate")
	require.NoError(t, err)
	defer func() { require.NoError(t, tempDB.Close()) }()

	// create a new satellitedb connection
	db, err := satellitedb.Open(ctx, log, tempDB.ConnStr, satellitedb.Options{
		ApplicationName: "satellite-migration-test",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	migration := getMigration(db)
	require.NoError(t, migration.Run(ctx, log))

	snapshot, err := pgutil.QuerySnapshot(ctx, db.Testing().RawDB())
	require.NoError(t, err)

	return migration.Steps[len(migration.Steps)-1].Version, snapshot
}

func BenchmarkSetup_Postgres(b *testing.B) {
	connstr := dbtest.PickPostgres(b)
	b.Run("merged", func(b *testing.B) {
		benchmarkSetup(b, connstr, true)
	})
	b.Run("separate", func(b *testing.B) {
		benchmarkSetup(b, connstr, false)
	})
}

func BenchmarkSetup_Cockroach(b *testing.B) {
	connstr := dbtest.PickCockroach(b)
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
			db, err := satellitedb.Open(ctx, log, tempDB.ConnStr, satellitedb.Options{ApplicationName: "satellite-migration-test"})
			require.NoError(b, err)
			defer func() { require.NoError(b, db.Close()) }()

			if merged {
				err = db.Testing().TestMigrateToLatest(ctx)
				require.NoError(b, err)
			} else {
				err = db.MigrateToLatest(ctx)
				require.NoError(b, err)
			}
		}()
	}
}
