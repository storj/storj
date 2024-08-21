// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil/pgerrcode"
)

func TestNow(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		sysnow := time.Now()
		now, err := db.Now(ctx)
		require.NoError(t, err)
		require.WithinDuration(t, sysnow, now, 5*time.Second)
	})
}

func TestFullMigration(t *testing.T) {
	migration := func(ctx context.Context, db *metabase.DB) error {
		return db.MigrateToLatest(ctx)
	}
	metabasetest.RunWithConfigAndMigration(t, metabase.Config{
		ApplicationName: "test-full-migration",
	}, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		sysnow := time.Now()
		now, err := db.Now(ctx)
		require.NoError(t, err)
		require.WithinDuration(t, sysnow, now, 5*time.Second)
	}, migration)
}

func TestDisallowDoubleUnversioned(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() == dbutil.Spanner {
			// TODO(spanner): implement one unversioned per location constraint for spanner.
			t.Skip("not implemented for spanner")
		}

		// This checks that TestingUniqueUnversioned=true indeed works as needed.
		objStream := metabasetest.RandObjectStream()
		obj := metabasetest.CreateObject(ctx, t, db, objStream, 0)

		object := metabase.RawObject{
			ObjectStream: metabase.ObjectStream{
				ProjectID:  obj.ProjectID,
				BucketName: obj.BucketName,
				ObjectKey:  obj.ObjectKey,
				Version:    obj.Version + 1,
				StreamID:   testrand.UUID(),
			},
			Status: metabase.CommittedUnversioned,
		}

		err := db.TestingBatchInsertObjects(ctx, []metabase.RawObject{object})

		require.True(t, pgerrcode.IsConstraintViolation(err))
		require.ErrorContains(t, err, "objects_one_unversioned_per_location")

		object.Status = metabase.DeleteMarkerUnversioned
		err = db.TestingBatchInsertObjects(ctx, []metabase.RawObject{object})

		require.True(t, pgerrcode.IsConstraintViolation(err))
		require.ErrorContains(t, err, "objects_one_unversioned_per_location")

		metabasetest.Verify{
			Objects: []metabase.RawObject{
				metabase.RawObject(obj),
			},
		}.Check(ctx, t, db)
	})
}

func TestSpannerProjects(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	connUrls := []string{}
	spannerFound := false
	for _, db := range satellitedbtest.Databases() {
		connUrls = append(connUrls, db.MetabaseDB.URL)

		if db.Name == "Spanner" {
			spannerFound = true
		}

	}
	require.Len(t, connUrls, 2, "two databases are required for this test")
	require.True(t, spannerFound, "Spanner must be configured for this test")

	spannerProjects := []uuid.UUID{testrand.UUID(), testrand.UUID()}
	nonSpannerProject := []uuid.UUID{testrand.UUID(), testrand.UUID()}

	log := zaptest.NewLogger(t)
	db, err := metabase.Open(ctx, log.Named("metabase"), strings.Join(connUrls, ";"), metabase.Config{
		ApplicationName:        "test-spanner-projects",
		TestingSpannerProjects: spannerProjects,
	})
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	for _, projectID := range spannerProjects {
		adapter := db.ChooseAdapter(projectID)
		_, ok := adapter.(*metabase.SpannerAdapter)
		require.True(t, ok, "project %v should be a spanner project", projectID)
	}

	for _, projectID := range nonSpannerProject {
		adapter := db.ChooseAdapter(projectID)
		_, ok := adapter.(*metabase.SpannerAdapter)
		require.False(t, ok, "project %v should NOT be a spanner project", projectID)
	}
}
