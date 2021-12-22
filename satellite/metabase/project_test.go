// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestGetProjectSegmentsCount(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		t.Run("ProjectID missing", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetProjectSegmentCount{
				Opts:     metabase.GetProjectSegmentCount{},
				ErrClass: &metabase.ErrInvalidRequest,
				ErrText:  "ProjectID missing",
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("empty database", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetProjectSegmentCount{
				Opts: metabase.GetProjectSegmentCount{
					ProjectID: obj.ProjectID,
				},
				ErrClass: &metabase.Error,
				ErrText:  "project not found: " + obj.ProjectID.String(),
			}.Check(ctx, t, db)
		})

		t.Run("object without segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj, 0)

			metabasetest.GetProjectSegmentCount{
				Opts: metabase.GetProjectSegmentCount{
					ProjectID: obj.ProjectID,
				},
				Result: 0,
			}.Check(ctx, t, db)
		})

		t.Run("object with segments", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj, 1)

			metabasetest.GetProjectSegmentCount{
				Opts: metabase.GetProjectSegmentCount{
					ProjectID: obj.ProjectID,
				},
				Result: 1,
			}.Check(ctx, t, db)
		})

		t.Run("object with segments (as of system time)", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.CreateObject(ctx, t, db, obj, 1)

			metabasetest.GetProjectSegmentCount{
				Opts: metabase.GetProjectSegmentCount{
					ProjectID:          obj.ProjectID,
					AsOfSystemTime:     time.Now(),
					AsOfSystemInterval: time.Millisecond,
				},
				Result: 1,
			}.Check(ctx, t, db)
		})

		t.Run("multiple projects", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			project1 := testrand.UUID()
			project2 := testrand.UUID()
			project3 := testrand.UUID()

			type Object struct {
				ProjectID uuid.UUID
				Segments  byte
			}

			objects := []Object{
				{project1, 1},
				{project1, 4},

				{project2, 5},
				{project2, 3},
				{project2, 1},

				{project3, 2},
				{project3, 0},
			}

			for _, object := range objects {
				obj := metabasetest.RandObjectStream()
				obj.ProjectID = object.ProjectID
				metabasetest.CreateObject(ctx, t, db, obj, object.Segments)
			}

			// expected number of segments per project
			projects := map[uuid.UUID]int{
				project1: 5,
				project2: 9,
				project3: 2,
			}

			for project, segments := range projects {
				metabasetest.GetProjectSegmentCount{
					Opts: metabase.GetProjectSegmentCount{
						ProjectID: project,
					},
					Result: int64(segments),
				}.Check(ctx, t, db)
			}
		})
	})
}
