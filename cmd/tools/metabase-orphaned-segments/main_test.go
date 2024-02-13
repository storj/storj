// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/dbutil"
	"storj.io/common/dbutil/tempdb"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	cmd "storj.io/storj/cmd/tools/metabase-orphaned-segments"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func Test_OrphanedSegment(t *testing.T) {
	os := metabasetest.RandObjectStream()
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB) {
		metabasetest.CreateObject(ctx, t, metabaseDB, os, 1)

		obj := metabasetest.CreateObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(), 10)
		_, err := rawDB.ExecContext(ctx, `DELETE FROM objects WHERE stream_id = $1`, obj.StreamID)
		require.NoError(t, err)

		objects, err := metabaseDB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 1)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 11)
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.Equal(t, os.StreamID, segments[0].StreamID)
	}
	test(t, prepare, check)
}

func Test_NoOrphanedSegment(t *testing.T) {
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB) {
		for i := 0; i < 14; i++ {
			metabasetest.CreateObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(), 1)
		}

		objects, err := metabaseDB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 14)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 14)
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 14)
	}
	test(t, prepare, check)
}

func Test_ManyOrphanedSegment(t *testing.T) {
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB) {
		metabasetest.CreateObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(), 1)

		for i := 0; i < 13; i++ {
			obj := metabasetest.CreateObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(), 1)
			_, err := rawDB.ExecContext(ctx, `DELETE FROM objects WHERE stream_id = $1`, obj.StreamID)
			require.NoError(t, err)
		}

		objects, err := metabaseDB.TestingAllObjects(ctx)
		require.NoError(t, err)
		require.Len(t, objects, 1)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 14)
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
	}
	test(t, prepare, check)
}

func test(t *testing.T, prepare func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB),
	check func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB)) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)

			metabaseTempDB, err := tempdb.OpenUnique(ctx, satelliteDB.MetabaseDB.URL, schema)
			require.NoError(t, err)

			metabaseDB, err := satellitedbtest.CreateMetabaseDBOnTopOf(ctx, log, metabaseTempDB, metabase.Config{
				ApplicationName:  "satellite-test",
				MinPartSize:      5 * memory.MiB,
				MaxNumberOfParts: 10000,
			})
			require.NoError(t, err)
			defer ctx.Check(metabaseDB.Close)

			err = metabaseDB.TestMigrateToLatest(ctx)
			require.NoError(t, err)

			prepare(t, ctx, metabaseTempDB, metabaseDB)

			cockroach := strings.HasPrefix(metabaseTempDB.ConnStr, "cockroach")

			// TODO workaround for pgx
			mConnStr := strings.Replace(metabaseTempDB.ConnStr, "cockroach", "postgres", 1)
			err = cmd.Delete(ctx, log, cmd.Config{
				MetabaseDB:      mConnStr,
				LoopBatchSize:   3,
				DeleteBatchSize: 2,
				Cockroach:       cockroach,
			})
			require.NoError(t, err)

			check(t, ctx, metabaseDB)
		})
	}
}
