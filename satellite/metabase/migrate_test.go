// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/dbutil/pgutil"
)

func TestMigration(t *testing.T) {
	for _, dbinfo := range satellitedbtest.Databases() {
		t.Run(dbinfo.Name, func(t *testing.T) {

			ctx := testcontext.NewWithTimeout(t, 8*time.Minute)
			defer ctx.Cleanup()

			prodSnapshot := schemaFromMigration(t, ctx, dbinfo.MetabaseDB, func(ctx context.Context, db *metabase.DB) error {
				return db.MigrateToLatest(ctx)
			})

			testSnapshot := schemaFromMigration(t, ctx, dbinfo.MetabaseDB, func(ctx context.Context, db *metabase.DB) error {
				return db.TestMigrateToLatest(ctx)
			})

			prodSnapshot.DropTable("metabase_versions")
			testSnapshot.DropTable("metabase_versions")

			require.Equal(t, prodSnapshot.Schema, testSnapshot.Schema, "Test snapshot scheme doesn't match the migrated scheme.")
			require.Equal(t, prodSnapshot.Data, testSnapshot.Data, "Test snapshot data doesn't match the migrated data.")

		})
	}
}

func schemaFromMigration(t *testing.T, ctx *testcontext.Context, dbinfo satellitedbtest.Database, migration func(ctx context.Context, db *metabase.DB) error) (scheme *dbschema.Snapshot) {
	db, err := satellitedbtest.CreateMetabaseDB(ctx, zaptest.NewLogger(t), t.Name(), "M", 0, dbinfo, metabase.Config{
		ApplicationName: "migration",
	})
	require.NoError(t, err)

	defer ctx.Check(db.Close)

	err = migration(ctx, db)
	require.NoError(t, err)

	scheme, err = pgutil.QuerySnapshot(ctx, db.UnderlyingTagSQL())
	require.NoError(t, err)

	return scheme
}
