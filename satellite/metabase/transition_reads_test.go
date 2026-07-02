// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestTransitionReads_GetObjectExactVersion(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("P served from primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			obj, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
			require.Equal(t, stream.StreamID, obj.StreamID)
		})

		t.Run("S fallback to secondary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			obj, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
			require.Equal(t, stream.StreamID, obj.StreamID)
		})

		t.Run("B prefers primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			// seed the same location in both backends, with different streamIDs.
			primaryStream := transitionSeedCommitted(ctx, t, primary, projectID, "both")
			secondaryStream := transitionSeedCommitted(ctx, t, secondary, projectID, "both")
			require.NotEqual(t, primaryStream.StreamID, secondaryStream.StreamID)

			obj, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
				ObjectLocation: primaryStream.Location(),
				Version:        primaryStream.Version,
			})
			require.NoError(t, err)
			require.Equal(t, primaryStream.StreamID, obj.StreamID)
		})

		t.Run("N not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			loc := metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: "bucket",
				ObjectKey:  "missing",
			}

			_, err := db.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
				ObjectLocation: loc,
				Version:        1,
			})
			require.True(t, metabase.ErrObjectNotFound.Has(err))
		})
	})
}

func TestTransitionReads_GetObjectLastCommitted(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("P served from primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			obj, err := db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
			require.Equal(t, stream.StreamID, obj.StreamID)
		})

		t.Run("S fallback to secondary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			obj, err := db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
			require.Equal(t, stream.StreamID, obj.StreamID)
		})

		t.Run("B prefers primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			primaryStream := transitionSeedCommitted(ctx, t, primary, projectID, "both")
			secondaryStream := transitionSeedCommitted(ctx, t, secondary, projectID, "both")
			require.NotEqual(t, primaryStream.StreamID, secondaryStream.StreamID)

			obj, err := db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
				ObjectLocation: primaryStream.Location(),
			})
			require.NoError(t, err)
			require.Equal(t, primaryStream.StreamID, obj.StreamID)
		})

		t.Run("N not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			loc := metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: "bucket",
				ObjectKey:  "missing",
			}

			_, err := db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
				ObjectLocation: loc,
			})
			require.True(t, metabase.ErrObjectNotFound.Has(err))
		})
	})
}

// Note: lock state (retention / legal hold) cannot be seeded via
// TestingBatchInsertObjects — it does not write the retention/legal-hold
// columns — so the lock-getter tests assert routing only: the getter resolves
// against whichever backend owns the object, and returns not-found otherwise.
// The lock values themselves are covered by the object-lock test suites.

func TestTransitionReads_GetObjectExactVersionRetention(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("P served from primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			_, err := db.GetObjectExactVersionRetention(ctx, metabase.GetObjectExactVersionRetention{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
		})

		t.Run("S fallback to secondary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			_, err := db.GetObjectExactVersionRetention(ctx, metabase.GetObjectExactVersionRetention{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
		})

		t.Run("N not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			_, err := db.GetObjectExactVersionRetention(ctx, metabase.GetObjectExactVersionRetention{
				ObjectLocation: metabase.ObjectLocation{ProjectID: projectID, BucketName: "bucket", ObjectKey: "missing"},
				Version:        1,
			})
			require.True(t, metabase.ErrObjectNotFound.Has(err))
		})
	})
}

func TestTransitionReads_GetObjectLastCommittedRetention(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("P served from primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			_, err := db.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
		})

		t.Run("S fallback to secondary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			_, err := db.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
		})

		t.Run("N not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			_, err := db.GetObjectLastCommittedRetention(ctx, metabase.GetObjectLastCommittedRetention{
				ObjectLocation: metabase.ObjectLocation{ProjectID: projectID, BucketName: "bucket", ObjectKey: "missing"},
			})
			require.True(t, metabase.ErrObjectNotFound.Has(err))
		})
	})
}

func TestTransitionReads_GetObjectExactVersionLegalHold(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("P served from primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			_, err := db.GetObjectExactVersionLegalHold(ctx, metabase.GetObjectExactVersionLegalHold{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
		})

		t.Run("S fallback to secondary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			_, err := db.GetObjectExactVersionLegalHold(ctx, metabase.GetObjectExactVersionLegalHold{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
		})

		t.Run("N not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			_, err := db.GetObjectExactVersionLegalHold(ctx, metabase.GetObjectExactVersionLegalHold{
				ObjectLocation: metabase.ObjectLocation{ProjectID: projectID, BucketName: "bucket", ObjectKey: "missing"},
				Version:        1,
			})
			require.True(t, metabase.ErrObjectNotFound.Has(err))
		})
	})
}

func TestTransitionReads_GetObjectLastCommittedLegalHold(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("P served from primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			_, err := db.GetObjectLastCommittedLegalHold(ctx, metabase.GetObjectLastCommittedLegalHold{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
		})

		t.Run("S fallback to secondary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			_, err := db.GetObjectLastCommittedLegalHold(ctx, metabase.GetObjectLastCommittedLegalHold{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
		})

		t.Run("N not found", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			_, err := db.GetObjectLastCommittedLegalHold(ctx, metabase.GetObjectLastCommittedLegalHold{
				ObjectLocation: metabase.ObjectLocation{ProjectID: projectID, BucketName: "bucket", ObjectKey: "missing"},
			})
			require.True(t, metabase.ErrObjectNotFound.Has(err))
		})
	})
}

func TestTransitionReads_BucketEmpty(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		bucketEmpty := func() bool {
			empty, err := db.BucketEmpty(ctx, metabase.BucketEmpty{
				ProjectID:  projectID,
				BucketName: "bucket",
			})
			require.NoError(t, err)
			return empty
		}

		t.Run("N empty in both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			require.True(t, bucketEmpty())
		})

		t.Run("P object only in primary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			transitionSeedCommitted(ctx, t, primary, projectID, "p-only")
			require.False(t, bucketEmpty())
		})

		t.Run("S object only in secondary", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")
			require.False(t, bucketEmpty())
		})

		t.Run("B object in both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)
			transitionSeedCommitted(ctx, t, primary, projectID, "p")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s")
			require.False(t, bucketEmpty())
		})
	})
}
