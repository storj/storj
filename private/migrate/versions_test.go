// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/tempdb"
	"storj.io/storj/shared/tagsql"
)

func TestBasicMigrationSqliteNoRebind(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	basicMigration(ctx, t, db, db)
}

func TestBasicMigrationSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	basicMigration(ctx, t, db, &sqliteDB{DB: db})
}

func TestBasicMigration(t *testing.T) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connstr, "create-", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { assert.NoError(t, db.Close()) }()

		basicMigration(ctx, t, db.DB, &postgresDB{DB: db.DB})
	})
}

func basicMigration(ctx *testcontext.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + strings.ReplaceAll(t.Name(), "/", "_"))
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName, "users")) }()

	/* #nosec G306 */ // This is a test besides the file contains just test data.
	err := os.WriteFile(ctx.File("alpha.txt"), []byte("test"), 0o644)
	require.NoError(t, err)
	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          &testDB,
				Description: "Initialize Table",
				Version:     1,
				Action: migrate.SQL{
					`CREATE TABLE users (id int)`,
					`INSERT INTO users (id) VALUES (1)`,
				},
			},
			{
				DB:          &testDB,
				Description: "Move files",
				Version:     2,
				Action: migrate.Func(func(_ context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					return os.Rename(ctx.File("alpha.txt"), ctx.File("beta.txt"))
				}),
			},
		},
	}

	dbVersion, err := m.CurrentVersion(ctx, nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, -1)

	err = m.Run(ctx, zap.NewNop())
	assert.NoError(t, err)

	dbVersion, err = m.CurrentVersion(ctx, nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, 2)

	m2 := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:      &testDB,
				Version: 3,
			},
		},
	}
	dbVersion, err = m2.CurrentVersion(ctx, nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, 2)

	var version int
	/* #nosec G202 */ // This is a test besides the dbName value is generated in
	// a controlled way
	err = db.QueryRowContext(ctx, `SELECT MAX(version) FROM `+dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	var id int
	err = db.QueryRowContext(ctx, `SELECT MAX(id) FROM users`).Scan(&id)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)

	// file not exists
	_, err = os.Stat(ctx.File("alpha.txt"))
	assert.Error(t, err)

	// file exists
	_, err = os.Stat(ctx.File("beta.txt"))
	assert.NoError(t, err)
	data, err := os.ReadFile(ctx.File("beta.txt"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("test"), data)
}

func TestMultipleMigrationSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	multipleMigration(ctx, t, db, &sqliteDB{DB: db})
}

func TestMultipleMigrationPostgres(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	connstr := dbtest.PickPostgres(t)

	db, err := tagsql.Open(ctx, "pgx", connstr, nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	multipleMigration(ctx, t, db, &postgresDB{DB: db})
}

func multipleMigration(ctx context.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + t.Name())
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName)) }()

	steps := 0
	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          &testDB,
				Description: "Step 1",
				Version:     1,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					steps++
					return nil
				}),
			},
			{
				DB:          &testDB,
				Description: "Step 2",
				Version:     2,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					steps++
					return nil
				}),
			},
		},
	}

	err := m.Run(ctx, zap.NewNop())
	assert.NoError(t, err)
	assert.Equal(t, 2, steps)

	m.Steps = append(m.Steps, &migrate.Step{
		DB:          &testDB,
		Description: "Step 3",
		Version:     3,
		Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
			steps++
			return nil
		}),
	})
	err = m.Run(ctx, zap.NewNop())
	assert.NoError(t, err)

	var version int
	/* #nosec G202 */ // This is a test besides the dbName value is generated in
	// a controlled way
	err = db.QueryRowContext(ctx, `SELECT MAX(version) FROM `+dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 3, version)

	assert.Equal(t, 3, steps)
}

func TestFailedMigrationSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	failedMigration(ctx, t, db, &sqliteDB{DB: db})
}

func TestFailedMigrationPostgres(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	connstr := dbtest.PickPostgres(t)

	db, err := tagsql.Open(ctx, "pgx", connstr, nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	failedMigration(ctx, t, db, &postgresDB{DB: db})
}

func failedMigration(ctx context.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + t.Name())
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName)) }()

	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          &testDB,
				Description: "Step 1",
				Version:     1,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					return errors.New("migration failed")
				}),
			},
		},
	}

	err := m.Run(ctx, zap.NewNop())
	require.Error(t, err, "migration failed")

	var version sql.NullInt64
	/* #nosec G202 */ // This is a test besides the dbName value is generated in
	// a controlled way
	err = db.QueryRowContext(ctx, `SELECT MAX(version) FROM `+dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, false, version.Valid)
}

func TestTargetVersion(t *testing.T) {
	m := migrate.Migration{
		Table: "test",
		Steps: []*migrate.Step{
			{
				Description: "Step 1",
				Version:     1,
				Action:      migrate.SQL{},
			},
			{
				Description: "Step 2",
				Version:     2,
				Action:      migrate.SQL{},
			},
			{
				Description: "Step 2.2",
				Version:     2,
				Action:      migrate.SQL{},
			},
			{
				Description: "Step 3",
				Version:     3,
				Action:      migrate.SQL{},
			},
		},
	}
	testedMigration := m.TargetVersion(2)
	assert.Equal(t, 3, len(testedMigration.Steps))
}

func TestInvalidStepsOrder(t *testing.T) {
	m := migrate.Migration{
		Table: "test",
		Steps: []*migrate.Step{
			{
				Version: 0,
			},
			{
				Version: 1,
			},
			{
				Version: 4,
			},
			{
				Version: 2,
			},
		},
	}

	err := m.ValidateSteps()
	require.Error(t, err, "migrate: steps have incorrect order")
}

func dropTables(ctx context.Context, db tagsql.DB, names ...string) error {
	var errlist errs.Group
	for _, name := range names {
		_, err := db.ExecContext(ctx, `DROP TABLE `+name)
		errlist.Add(err)
	}

	return errlist.Err()
}
