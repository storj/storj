// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestPrecommitConstraint_Empty(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		obj := metabasetest.RandObjectStream()

		for _, versioned := range []bool{false, true} {
			for _, disallowDelete := range []bool{false, true} {
				name := fmt.Sprintf("Versioned:%v,DisallowDelete:%v", versioned, disallowDelete)
				t.Run(name, func(t *testing.T) {
					result, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
						Location:       obj.Location(),
						Versioned:      versioned,
						DisallowDelete: disallowDelete,
					}, db.UnderlyingTagSQL())
					require.NoError(t, err)
					require.Equal(t, metabase.PrecommitConstraintResult{}, result)
				})
			}
		}

		t.Run("with-non-pending", func(t *testing.T) {
			result, err := db.PrecommitDeleteUnversionedWithNonPending(ctx, obj.Location(), db.UnderlyingTagSQL())
			require.NoError(t, err)
			require.Equal(t, metabase.PrecommitConstraintWithNonPendingResult{}, result)
		})
	})
}

func BenchmarkPrecommitConstraint(b *testing.B) {
	metabasetest.Bench(b, func(ctx *testcontext.Context, b *testing.B, db *metabase.DB) {
		baseObj := metabasetest.RandObjectStream()

		for i := 0; i < 500; i++ {
			metabasetest.CreateObject(ctx, b, db, metabasetest.RandObjectStream(), 0)
		}

		for i := 0; i < 10; i++ {
			baseObj.ObjectKey = metabase.ObjectKey("foo/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)

			baseObj.ObjectKey = metabase.ObjectKey("foo/prefixA/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)

			baseObj.ObjectKey = metabase.ObjectKey("foo/prefixB/" + strconv.Itoa(i))
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)
		}

		for i := 0; i < 50; i++ {
			baseObj.ObjectKey = metabase.ObjectKey("boo/foo" + strconv.Itoa(i) + "/object")
			metabasetest.CreateObject(ctx, b, db, baseObj, 0)
		}

		b.Run("unversioned", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
					Location: metabase.ObjectLocation{
						ProjectID:  baseObj.ProjectID,
						BucketName: baseObj.BucketName,
						ObjectKey:  "foo/5",
					},
					Versioned:      false,
					DisallowDelete: false,
				}, db.UnderlyingTagSQL())
				require.NoError(b, err)
			}
		})

		b.Run("versioned", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := db.PrecommitConstraint(ctx, metabase.PrecommitConstraint{
					Location: metabase.ObjectLocation{
						ProjectID:  baseObj.ProjectID,
						BucketName: baseObj.BucketName,
						ObjectKey:  "foo/5",
					},
					Versioned:      true,
					DisallowDelete: false,
				}, db.UnderlyingTagSQL())
				require.NoError(b, err)
			}
		})
	})
}
