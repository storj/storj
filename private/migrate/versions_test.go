// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/private/dbutil/tempdb"
	"storj.io/storj/private/migrate"
)

func TestBasicMigrationSqliteNoRebind(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	basicMigration(t, db, db)
}

func TestBasicMigrationSqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	basicMigration(t, db, &sqliteDB{DB: db})
}

func TestBasicMigrationPostgres(t *testing.T) {
	if *pgtest.ConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", pgtest.DefaultConnStr)
	}
	testBasicMigrationGeneric(t, *pgtest.ConnStr)
}

func TestBasicMigrationCockroach(t *testing.T) {
	if *pgtest.CrdbConnStr == "" {
		t.Skipf("cockroach flag missing, example:\n-cockroach-test-db=%s", pgtest.DefaultCrdbConnStr)
	}
	testBasicMigrationGeneric(t, *pgtest.CrdbConnStr)
}

func testBasicMigrationGeneric(t *testing.T, connStr string) {
	db, err := tempdb.OpenUnique(connStr, "create-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { assert.NoError(t, db.Close()) }()

	basicMigration(t, db.DB, &postgresDB{DB: db.DB})
}

func basicMigration(t *testing.T, db *sql.DB, testDB migrate.DB) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dbName := strings.ToLower(`versions_` + t.Name())
	defer func() { assert.NoError(t, dropTables(db, dbName, "users")) }()

	err := ioutil.WriteFile(ctx.File("alpha.txt"), []byte("test"), 0644)
	require.NoError(t, err)
	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          testDB,
				Description: "Initialize Table",
				Version:     1,
				Action: migrate.SQL{
					`CREATE TABLE users (id int)`,
					`INSERT INTO users (id) VALUES (1)`,
				},
			},
			{
				DB:          testDB,
				Description: "Move files",
				Version:     2,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					return os.Rename(ctx.File("alpha.txt"), ctx.File("beta.txt"))
				}),
			},
		},
	}

	dbVersion, err := m.CurrentVersion(nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, -1)

	err = m.Run(zap.NewNop())
	assert.NoError(t, err)

	dbVersion, err = m.CurrentVersion(nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, 2)

	m2 := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:      testDB,
				Version: 3,
			},
		},
	}
	dbVersion, err = m2.CurrentVersion(nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, 2)

	var version int
	err = db.QueryRow(`SELECT MAX(version) FROM ` + dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	var id int
	err = db.QueryRow(`SELECT MAX(id) FROM users`).Scan(&id)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)

	// file not exists
	_, err = os.Stat(ctx.File("alpha.txt"))
	assert.Error(t, err)

	// file exists
	_, err = os.Stat(ctx.File("beta.txt"))
	assert.NoError(t, err)
	data, err := ioutil.ReadFile(ctx.File("beta.txt"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("test"), data)
}

func TestMultipleMigrationSqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	multipleMigration(t, db, &sqliteDB{DB: db})
}

func TestMultipleMigrationPostgres(t *testing.T) {
	if *pgtest.ConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", pgtest.DefaultConnStr)
	}

	db, err := sql.Open("postgres", *pgtest.ConnStr)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	multipleMigration(t, db, &postgresDB{DB: db})
}

func multipleMigration(t *testing.T, db *sql.DB, testDB migrate.DB) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dbName := strings.ToLower(`versions_` + t.Name())
	defer func() { assert.NoError(t, dropTables(db, dbName)) }()

	steps := 0
	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          testDB,
				Description: "Step 1",
				Version:     1,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					steps++
					return nil
				}),
			},
			{
				DB:          testDB,
				Description: "Step 2",
				Version:     2,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					steps++
					return nil
				}),
			},
		},
	}

	err := m.Run(zap.NewNop())
	assert.NoError(t, err)
	assert.Equal(t, 2, steps)

	m.Steps = append(m.Steps, &migrate.Step{
		DB:          testDB,
		Description: "Step 3",
		Version:     3,
		Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
			steps++
			return nil
		}),
	})
	err = m.Run(zap.NewNop())
	assert.NoError(t, err)

	var version int
	err = db.QueryRow(`SELECT MAX(version) FROM ` + dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 3, version)

	assert.Equal(t, 3, steps)
}

func TestFailedMigrationSqlite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	failedMigration(t, db, &sqliteDB{DB: db})
}

func TestFailedMigrationPostgres(t *testing.T) {
	if *pgtest.ConnStr == "" {
		t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", pgtest.DefaultConnStr)
	}

	db, err := sql.Open("postgres", *pgtest.ConnStr)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	failedMigration(t, db, &postgresDB{DB: db})
}

func failedMigration(t *testing.T, db *sql.DB, testDB migrate.DB) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dbName := strings.ToLower(`versions_` + t.Name())
	defer func() { assert.NoError(t, dropTables(db, dbName)) }()

	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          testDB,
				Description: "Step 1",
				Version:     1,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					return fmt.Errorf("migration failed")
				}),
			},
		},
	}

	err := m.Run(zap.NewNop())
	require.Error(t, err, "migration failed")

	var version sql.NullInt64
	err = db.QueryRow(`SELECT MAX(version) FROM ` + dbName).Scan(&version)
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

func dropTables(db *sql.DB, names ...string) error {
	var errlist errs.Group
	for _, name := range names {
		_, err := db.Exec(`DROP TABLE ` + name)
		errlist.Add(err)
	}

	return errlist.Err()
}
