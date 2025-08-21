// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

func TestGetTableStats(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() == dbutil.Spanner {
			// TODO(spanner): implement for spanner.
			t.Skip("not correct implementation for spanner")
		}

		t.Run("no data", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			metabasetest.GetTableStats{
				Result: metabase.TableStats{},
			}.Check(ctx, t, db)

			metabasetest.Verify{}.Check(ctx, t, db)
		})

		t.Run("data", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			obj1 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{}.Run(ctx, t, db, obj1, 4)

			metabasetest.GetTableStats{
				Result: metabase.TableStats{
					SegmentCount: 4,
				},
			}.Check(ctx, t, db)

			obj2 := metabasetest.RandObjectStream()
			metabasetest.CreateTestObject{}.Run(ctx, t, db, obj2, 3)

			metabasetest.GetTableStats{
				Result: metabase.TableStats{
					SegmentCount: 7,
				},
			}.Check(ctx, t, db)
		})

		if db.Implementation() == dbutil.Cockroach {
			t.Run("as of interval", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				metabasetest.GetTableStats{
					Opts: metabase.GetTableStats{
						AsOfSystemInterval: -time.Microsecond,
					},
					Result: metabase.TableStats{
						SegmentCount: 0,
					},
				}.Check(ctx, t, db)

				time.Sleep(2 * time.Second)

				obj1 := metabasetest.RandObjectStream()
				metabasetest.CreateTestObject{}.Run(ctx, t, db, obj1, 4)

				metabasetest.GetTableStats{
					Opts: metabase.GetTableStats{
						AsOfSystemInterval: -1 * time.Second,
					},
					Result: metabase.TableStats{
						SegmentCount: 0,
					},
				}.Check(ctx, t, db)

				metabasetest.GetTableStats{
					Opts: metabase.GetTableStats{
						AsOfSystemInterval: -time.Microsecond,
					},
					Result: metabase.TableStats{
						SegmentCount: 4,
					},
				}.Check(ctx, t, db)
			})

			t.Run("use statistics", func(t *testing.T) {
				defer metabasetest.DeleteAll{}.Check(ctx, t, db)

				obj1 := metabasetest.RandObjectStream()
				metabasetest.CreateTestObject{}.Run(ctx, t, db, obj1, 4)

				err := db.UpdateTableStats(ctx)
				require.NoError(t, err)

				// add some segments after creating statistics to know that results are taken
				// from statistics and not directly with SELECT count(*)
				obj1 = metabasetest.RandObjectStream()
				metabasetest.CreateTestObject{}.Run(ctx, t, db, obj1, 4)

				metabasetest.GetTableStats{
					Opts: metabase.GetTableStats{},
					Result: metabase.TableStats{
						SegmentCount: 4,
					},
				}.Check(ctx, t, db)
			})
		}
	})
}

func TestCountSegments(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		if db.Implementation() != dbutil.Spanner {
			t.Skip("implemented only for spanner")
		}

		metabasetest.CreateObject(ctx, t, db, metabasetest.RandObjectStream(), 4)

		now, err := db.Now(ctx)
		require.NoError(t, err)

		result, err := db.CountSegments(ctx, now)
		require.NoError(t, err)
		require.EqualValues(t, 4, result.SegmentCount)
		require.EqualValues(t, []int64{4}, result.PerAdapterSegmentCount)
	})
}
