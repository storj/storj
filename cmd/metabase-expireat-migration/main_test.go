// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/tempdb"
	migrator "storj.io/storj/cmd/metabase-expireat-migration"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestMigrator_NoSegments(t *testing.T) {
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB) {
		metabasetest.CreateObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(), 0)
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 0)
	}
	test(t, prepare, check)
}

func TestMigrator_SingleSegment(t *testing.T) {
	expectedExpiresAt := time.Now().Add(27 * time.Hour)
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB) {
		metabasetest.CreateExpiredObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(),
			1, expectedExpiresAt)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.NotNil(t, segments[0].ExpiresAt)

		_, err = rawDB.ExecContext(ctx, `UPDATE segments SET expires_at = NULL`)
		require.NoError(t, err)

		segments, err = metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.Nil(t, segments[0].ExpiresAt)
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 1)
		require.NotNil(t, segments[0].ExpiresAt)
		require.Equal(t, expectedExpiresAt.Unix(), segments[0].ExpiresAt.Unix())
	}
	test(t, prepare, check)
}

func TestMigrator_ManySegments(t *testing.T) {
	numberOfObjects := 100
	expectedExpiresAt := map[uuid.UUID]*time.Time{}
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB) {
		for i := 0; i < numberOfObjects; i++ {
			commitedObject := metabasetest.CreateExpiredObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(),
				1, time.Now().Add(5*time.Hour))
			expectedExpiresAt[commitedObject.StreamID] = commitedObject.ExpiresAt
		}

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, numberOfObjects)
		for _, segment := range segments {
			require.NotNil(t, segment.ExpiresAt)
		}

		_, err = rawDB.ExecContext(ctx, `UPDATE segments SET expires_at = NULL`)
		require.NoError(t, err)

		segments, err = metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, numberOfObjects)
		for _, segment := range segments {
			require.Nil(t, segment.ExpiresAt)
		}
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, numberOfObjects)
		for _, segment := range segments {
			require.NotNil(t, segment.ExpiresAt)
			expiresAt, found := expectedExpiresAt[segment.StreamID]
			require.True(t, found)
			require.Equal(t, expiresAt, segment.ExpiresAt)
		}
	}
	test(t, prepare, check)
}

func TestMigrator_SegmentsWithAndWithoutExpiresAt(t *testing.T) {
	expectedExpiresAt := time.Now().Add(27 * time.Hour)
	var segmentsBefore []metabase.Segment
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, metabaseDB *metabase.DB) {
		metabasetest.CreateExpiredObject(ctx, t, metabaseDB, metabasetest.RandObjectStream(),
			10, expectedExpiresAt)

		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 10)
		for _, segment := range segments {
			require.NotNil(t, segment.ExpiresAt)
		}

		// set expires_at to null for half of segments
		_, err = rawDB.ExecContext(ctx, `UPDATE segments SET expires_at = NULL WHERE position < 5`)
		require.NoError(t, err)

		segmentsBefore, err = metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segmentsBefore, 10)
		for i := 0; i < len(segmentsBefore); i++ {
			if i < 5 {
				require.Nil(t, segmentsBefore[i].ExpiresAt)
			} else {
				require.NotNil(t, segmentsBefore[i].ExpiresAt)
			}
		}
	}

	check := func(t *testing.T, ctx context.Context, metabaseDB *metabase.DB) {
		segments, err := metabaseDB.TestingAllSegments(ctx)
		require.NoError(t, err)
		require.Len(t, segments, 10)
		for i := 0; i < len(segments); i++ {
			require.NotNil(t, segments[i].ExpiresAt)
			require.Equal(t, expectedExpiresAt.Unix(), segments[i].ExpiresAt.Unix())
		}
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

			metabaseDB, err := satellitedbtest.CreateMetabaseDBOnTopOf(ctx, log, metabaseTempDB)
			require.NoError(t, err)
			defer ctx.Check(metabaseDB.Close)

			err = metabaseDB.MigrateToLatest(ctx)
			require.NoError(t, err)

			prepare(t, ctx, metabaseTempDB, metabaseDB)

			cockroach := strings.HasPrefix(metabaseTempDB.ConnStr, "cockroach")

			// TODO workaround for pgx
			mConnStr := strings.Replace(metabaseTempDB.ConnStr, "cockroach", "postgres", 1)
			err = migrator.Migrate(ctx, log, migrator.Config{
				MetabaseDB:      mConnStr,
				LoopBatchSize:   40,
				UpdateBatchSize: 10,
				Cockroach:       cockroach,
			})
			require.NoError(t, err)

			check(t, ctx, metabaseDB)
		})
	}
}
