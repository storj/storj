// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"strings"
	"testing"

	pgx "github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	migrator "storj.io/storj/cmd/tools/migrate-public-ids"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/tempdb"
)

// Test no entries in table doesn't error.
func TestMigrateProjectsSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB, conn *pgx.Conn, log *zap.Logger) {
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateProjects, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateProjectsTest(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var n int
	var notUpdate *console.Project
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB, conn *pgx.Conn, log *zap.Logger) {
		_, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		n++
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test1",
			Description: "test1",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		n++
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		n++
		notUpdate, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		err = testNullifyPublicIDs(ctx, log, conn, notUpdate.ID)
		require.NoError(t, err)

		projects, err := db.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		for _, p := range projects {
			if p.ID == notUpdate.ID {
				require.False(t, p.PublicID.IsZero())
			} else {
				require.True(t, p.PublicID.IsZero())
			}
		}
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		projects, err := db.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		var updated int
		var checkedNotUpdate bool
		publicIDs := make(map[uuid.UUID]bool)
		for _, prj := range projects {
			if prj.ID == notUpdate.ID {
				checkedNotUpdate = true
				require.Equal(t, notUpdate.PublicID, prj.PublicID)
			} else if !prj.PublicID.IsZero() {
				updated++
			}
			if _, ok := publicIDs[prj.ID]; !ok {
				publicIDs[prj.ID] = true
			} else {
				t.Fatalf("duplicate public_id: %v", prj.ID)
			}
		}
		require.Equal(t, n, updated)
		require.True(t, checkedNotUpdate)
		n = 0
		notUpdate = &console.Project{}
	}

	test(t, prepare, migrator.MigrateProjects, check, &migrator.Config{
		Limit: 2,
	})
}

func test(t *testing.T, prepare func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB, conn *pgx.Conn, log *zap.Logger),
	migrate func(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config migrator.Config) (err error),
	check func(t *testing.T, ctx context.Context, db satellite.DB), config *migrator.Config) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	for _, satelliteDB := range satellitedbtest.Databases(t) {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			if satelliteDB.Name == "Spanner" {
				t.Skip("not implemented for spanner")
			}

			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)

			tempDB, err := tempdb.OpenUnique(ctx, log, satelliteDB.MasterDB.URL, schema, satelliteDB.MasterDB.ExtraStatements)
			require.NoError(t, err)

			db, err := satellitedbtest.CreateMasterDBOnTopOf(ctx, log, tempDB, satellitedb.Options{
				ApplicationName: "migrate-public-ids",
			})
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			err = db.Testing().TestMigrateToLatest(ctx)
			require.NoError(t, err)

			mConnStr := strings.Replace(tempDB.ConnStr, "cockroach", "postgres", 1)

			conn, err := pgx.Connect(ctx, mConnStr)
			require.NoError(t, err)

			prepare(t, ctx, tempDB, db, conn, log)

			err = migrate(ctx, log, conn, *config)
			require.NoError(t, err)

			require.NoError(t, err)

			check(t, ctx, db)
		})
	}
}

// This is required to test the migration since now all projects are inserted with a public_id.
//
// * * * THIS IS ONLY FOR TESTING!!! * * *.
func testNullifyPublicIDs(ctx context.Context, log *zap.Logger, conn *pgx.Conn, exclude uuid.UUID) error {
	_, err := conn.Exec(ctx, `
		UPDATE projects
		SET public_id = NULL
		WHERE id != $1;
	`, exclude.Bytes())
	return err
}
