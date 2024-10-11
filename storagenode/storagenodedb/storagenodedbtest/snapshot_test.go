// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/tagsql"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/storagenodedb"
)

// TestSnapshot tests if the snapshot migration (used for faster testplanet) is the same as the prod migration.
func TestSnapshot(t *testing.T) {

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fromMigrationSteps := getSchemeSnapshot(t, ctx, "migration", func(ctx context.Context, db *storagenodedb.DB) error {
		return db.MigrateToLatest(ctx)
	})
	fromSnapshot := getSchemeSnapshot(t, ctx, "steps", func(ctx context.Context, db *storagenodedb.DB) error {
		if err := deploySnapshot(db.DBDirectory()); err != nil {
			return err
		}
		return db.MigrateToLatest(ctx)
	})

	require.Equal(t, fromSnapshot, fromMigrationSteps, "The database snapshot produces a different scheme than the current storagenodedb migrations. "+
		"If you have introduced a new migration, please run go generate ./storagenode/storagenodedb/storagenodedbtest/testdata to update the snapshot.")
}

func getSchemeSnapshot(t *testing.T, ctx *testcontext.Context, name string, init func(ctx context.Context, db *storagenodedb.DB) error) schemeSnapshot {
	log := zaptest.NewLogger(t)

	storageDir := ctx.Dir(name)
	cfg := storagenodedb.Config{
		Pieces:    storageDir,
		Storage:   storageDir,
		Info:      filepath.Join(storageDir, "piecestore.db"),
		Info2:     filepath.Join(storageDir, "info.db"),
		Filestore: filestore.DefaultConfig,
	}

	db, err := storagenodedb.OpenNew(ctx, log, cfg)
	if err != nil {
		require.NoError(t, err)
	}
	defer ctx.Check(db.Close)

	err = init(ctx, db)
	require.NoError(t, err)

	return getSerializedScheme(t, ctx, db)

}

// schemeSnapshot represents dbname -> scheme.
type schemeSnapshot map[string]dbScheme

// dbScheme represents uniq id (table/name/...) -> sql.
type dbScheme map[string]string

func getSerializedScheme(t *testing.T, ctx *testcontext.Context, db *storagenodedb.DB) schemeSnapshot {
	dbs := schemeSnapshot{}
	for dbName, db := range db.SQLDBs {
		s := dbScheme{}
		sqliteScheme, err := readSqliteScheme(ctx, db.GetDB())
		require.Nil(t, err)
		for k, v := range sqliteScheme {
			s[k] = v
		}

		dbs[dbName] = s
	}
	return dbs
}

func readSqliteScheme(ctx context.Context, db tagsql.DB) (map[string]string, error) {
	var root int
	var schemaType, name, table string
	var sqlContent sql.NullString

	res := map[string]string{}
	schema, err := db.QueryContext(ctx, "select * from sqlite_schema")
	if err != nil {
		return nil, errs.Combine(err, schema.Close())
	}

	for schema.Next() {
		if schema.Err() != nil {
			return nil, errs.Combine(schema.Err(), schema.Close())
		}
		err = schema.Scan(&schemaType, &name, &table, &root, &sqlContent)
		if err != nil {
			return nil, errs.Combine(err, schema.Close())
		}

		// due to the migration logic we will have separated version table for each db
		if name != "versions" {
			res[fmt.Sprintf("%s.%s.%s", schemaType, name, table)] = sqlContent.String
		}

	}
	return res, errs.Combine(schema.Err(), schema.Close())
}
