// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// TestExactVersionStreamIDSuffix checks that the operations on an exact version don't match an
// object whose stream ID doesn't end with the requested suffix, such as an object that was
// recreated with the version of one that was deleted.
func TestExactVersionStreamIDSuffix(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		defer metabasetest.DeleteAll{}.Check(ctx, t, db)

		objStream := metabasetest.RandObjectStream()
		object, _ := metabasetest.CreateTestObject{
			BeginObjectExactVersion: &metabase.BeginObjectExactVersion{
				ObjectStream: objStream,
				Encryption:   metabasetest.DefaultEncryption,
			},
			CommitObject: &metabase.CommitObject{
				ObjectStream: objStream,
				Versioned:    true,
			},
		}.Run(ctx, t, db, objStream, 0)

		suffix := objStream.StreamIDSuffix()
		otherSuffix := suffix
		otherSuffix[0]++

		t.Run("GetObjectExactVersion", func(t *testing.T) {
			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: otherSuffix,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersion{
				Opts: metabase.GetObjectExactVersion{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: suffix,
				},
				Result: object,
			}.Check(ctx, t, db)
		})

		t.Run("Legal hold", func(t *testing.T) {
			metabasetest.SetObjectExactVersionLegalHold{
				Opts: metabase.SetObjectExactVersionLegalHold{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: otherSuffix,
					Enabled:        true,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: otherSuffix,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionLegalHold{
				Opts: metabase.GetObjectExactVersionLegalHold{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: suffix,
				},
				Result: false,
			}.Check(ctx, t, db)
		})

		t.Run("Retention", func(t *testing.T) {
			metabasetest.SetObjectExactVersionRetention{
				Opts: metabase.SetObjectExactVersionRetention{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: otherSuffix,
					Retention: metabase.Retention{
						Mode:        storj.GovernanceMode,
						RetainUntil: time.Now().Add(time.Hour),
					},
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: otherSuffix,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			metabasetest.GetObjectExactVersionRetention{
				Opts: metabase.GetObjectExactVersionRetention{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: suffix,
				},
				Result: metabase.Retention{},
			}.Check(ctx, t, db)
		})

		t.Run("BeginCopyObject", func(t *testing.T) {
			metabasetest.BeginCopyObject{
				Opts: metabase.BeginCopyObject{
					ObjectLocation: objStream.Location(),
					Version:        objStream.Version,
					StreamIDSuffix: otherSuffix,
					SegmentLimit:   10,
				},
				ErrClass: &metabase.ErrObjectNotFound,
			}.Check(ctx, t, db)

			_, err := db.BeginCopyObject(ctx, metabase.BeginCopyObject{
				ObjectLocation: objStream.Location(),
				Version:        objStream.Version,
				StreamIDSuffix: suffix,
				SegmentLimit:   10,
			})
			require.NoError(t, err)
		})

		metabasetest.Verify{
			Objects: []metabase.RawObject{metabase.RawObject(object)},
		}.Check(ctx, t, db)
	})
}
