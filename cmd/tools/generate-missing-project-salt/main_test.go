// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"

	pgx "github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/tempdb"
	migrator "storj.io/storj/cmd/tools/generate-missing-project-salt"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Test salt column is updated correctly.
func TestGenerateMissingSaltTest(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var n int
	var projectsIDs []uuid.UUID
	var test2Salt uuid.UUID
	var salt2 []byte
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB, conn *pgx.Conn, log *zap.Logger) {
		myProject1, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test1",
			Description: "test1",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		projectsIDs = append(projectsIDs, myProject1.ID)
		err = db.Console().Projects().TestNullifySalt(ctx, myProject1.ID)
		require.NoError(t, err)
		salt1, err := db.Console().Projects().TestGetSalt(ctx, myProject1.ID)
		require.NoError(t, err)
		require.Equal(t, len(salt1), 0)
		n++

		// Project "test2" should have a populated salt column and should not get updated.
		myProject2, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test2",
			Description: "test2",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		projectsIDs = append(projectsIDs, myProject2.ID)
		test2Salt = myProject2.ID
		salt2, err = db.Console().Projects().TestGetSalt(ctx, myProject2.ID)
		require.NoError(t, err)
		require.NotNil(t, salt2)
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		var updated int
		var checkedNotUpdate bool

		for _, p := range projectsIDs {
			saltdb, err := db.Console().Projects().TestGetSalt(ctx, p)
			require.NoError(t, err)
			idHash := sha256.Sum256(p[:])
			salt := idHash[:]
			// The insert method creates a new uuid rather than the hashed project id for the salt column.
			// Test 2 was not updated, so the salt column will not match the hashed project id.
			if p == test2Salt {
				require.NotEqual(t, salt, saltdb)
				checkedNotUpdate = true
				require.Equal(t, salt2, saltdb)
			} else {
				require.Equal(t, salt, saltdb)
				updated++
			}
		}
		require.Equal(t, n, updated)
		require.True(t, checkedNotUpdate)
		n = 0
	}

	test(t, prepare, migrator.GenerateMissingSalt, check, &migrator.Config{
		Limit: 2,
	})
}

func test(t *testing.T, prepare func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB, conn *pgx.Conn, log *zap.Logger),
	migrate func(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config migrator.Config) (err error),
	check func(t *testing.T, ctx context.Context, db satellite.DB), config *migrator.Config) {

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)

			tempDB, err := tempdb.OpenUnique(ctx, satelliteDB.MasterDB.URL, schema)
			require.NoError(t, err)

			db, err := satellitedbtest.CreateMasterDBOnTopOf(ctx, log, tempDB)
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			err = db.TestingMigrateToLatest(ctx)
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
