// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

import (
	"database/sql"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/internal/testcontext"
)

// TODO multiple db tests
// TODO test failed migration

func TestBasicMigration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	defer func() { assert.NoError(t, db.Close()) }()

	testDB := &sqliteDB{DB: db}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	err = ioutil.WriteFile(ctx.File("alpha.txt"), []byte("test"), 0644)
	require.NoError(t, err)
	m := migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				Description: "Initialize Table",
				Version:     1,
				Action: migrate.SQL{
					`CREATE TABLE users (id int)`,
					`INSERT INTO users (id) VALUES (1)`,
				},
			},
			{
				Description: "Move files",
				Version:     2,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					return os.Rename(filepath.Join(ctx.Dir(), "alpha.txt"), filepath.Join(ctx.Dir(), "beta.txt"))
				}),
			},
		},
	}

	err = m.Run(zap.NewNop(), testDB)
	assert.NoError(t, err)

	var version int
	err = db.QueryRow(`SELECT MAX(version) FROM versions`).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	var id int
	err = db.QueryRow(`SELECT MAX(id) FROM users`).Scan(&id)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)

	// file not exists
	_, err = os.Stat(filepath.Join(ctx.Dir(), "alpha.txt"))
	assert.Error(t, err)

	// file exists
	_, err = os.Stat(filepath.Join(ctx.Dir(), "beta.txt"))
	assert.NoError(t, err)
}

func TestMultipleMigration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	defer func() { assert.NoError(t, db.Close()) }()

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testDB := &sqliteDB{DB: db}

	err = ioutil.WriteFile(ctx.File("alpha.txt"), []byte("test"), 0644)
	require.NoError(t, err)
	steps := 0
	m := migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				Description: "Step 1",
				Version:     1,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					steps++
					return nil
				}),
			},
			{
				Description: "Step 2",
				Version:     2,
				Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
					steps++
					return nil
				}),
			},
		},
	}

	err = m.Run(zap.NewNop(), testDB)
	assert.NoError(t, err)
	assert.Equal(t, 2, steps)

	m.Steps = append(m.Steps, &migrate.Step{
		Description: "Step 3",
		Version:     3,
		Action: migrate.Func(func(log *zap.Logger, _ migrate.DB, tx *sql.Tx) error {
			steps++
			return nil
		}),
	})
	err = m.Run(zap.NewNop(), testDB)
	assert.NoError(t, err)

	assert.Equal(t, 3, steps)
}
