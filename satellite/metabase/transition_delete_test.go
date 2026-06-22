// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// transitionDeleteSeedCommittedBoth seeds the identical committed unversioned
// object into both backends (the relocation-window scenario) and returns the
// shared stream.
func transitionDeleteSeedCommittedBoth(ctx *testcontext.Context, t *testing.T, primary, secondary metabase.Adapter, projectID uuid.UUID, key string) metabase.ObjectStream {
	stream := transitionSeedCommitted(ctx, t, primary, projectID, key)
	require.NoError(t, secondary.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.CommittedUnversioned,
		Encryption:   metabasetest.DefaultEncryption,
	}}))
	return stream
}

// transitionDeleteSeedPending inserts a pending object directly into the given
// backend, bypassing the transition routing.
func transitionDeleteSeedPending(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter, projectID uuid.UUID, key string) metabase.ObjectStream {
	stream := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: "bucket",
		ObjectKey:  metabase.ObjectKey(key),
		Version:    1,
		StreamID:   testrand.UUID(),
	}
	require.NoError(t, adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.Pending,
		Encryption:   metabasetest.DefaultEncryption,
	}}))
	return stream
}

// transitionDeleteSeedPendingBoth seeds the identical pending object into both
// backends and returns the shared stream.
func transitionDeleteSeedPendingBoth(ctx *testcontext.Context, t *testing.T, primary, secondary metabase.Adapter, projectID uuid.UUID, key string) metabase.ObjectStream {
	stream := transitionDeleteSeedPending(ctx, t, primary, projectID, key)
	require.NoError(t, secondary.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.Pending,
		Encryption:   metabasetest.DefaultEncryption,
	}}))
	return stream
}

// transitionDeleteObjectsInAdapter returns the objects currently stored in the
// given backend.
func transitionDeleteObjectsInAdapter(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter) []metabase.RawObject {
	objects, err := adapter.TestingGetAllObjects(ctx)
	require.NoError(t, err)
	return objects
}

// transitionDeleteExpiresAtSet reports whether the object with streamID exists
// in objects and has expires_at set (the soft-delete marker used for pending
// objects).
func transitionDeleteExpiresAtSet(objects []metabase.RawObject, streamID uuid.UUID) bool {
	for _, o := range objects {
		if o.StreamID == streamID {
			return o.ExpiresAt != nil
		}
	}
	return false
}

func TestTransitionDelete_DeleteObjectExactVersion(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			result, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
			require.Len(t, result.Removed, 1)
			require.Equal(t, stream.StreamID, result.Removed[0].StreamID)

			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			result, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
			require.Len(t, result.Removed, 1)
			require.Equal(t, stream.StreamID, result.Removed[0].StreamID)

			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
		})

		t.Run("both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionDeleteSeedCommittedBoth(ctx, t, primary, secondary, projectID, "both")

			result, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
				ObjectLocation: stream.Location(),
				Version:        stream.Version,
			})
			require.NoError(t, err)
			// merged result: one removed object from each backend.
			require.Len(t, result.Removed, 2)

			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
		})

		t.Run("neither", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			loc := metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: "bucket",
				ObjectKey:  "missing",
			}
			// The per-adapter DeleteObjectExactVersion returns an empty result
			// (not a not-found error) when nothing matches, so deleteBoth merges
			// two empty results and reports no error.
			result, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
				ObjectLocation: loc,
				Version:        1,
			})
			require.NoError(t, err)
			require.Empty(t, result.Removed)
		})
	})
}

func TestTransitionDelete_DeletePendingObject(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionDeleteSeedPending(ctx, t, primary, projectID, "p-only")

			result, err := db.DeletePendingObject(ctx, metabase.DeletePendingObject{
				ObjectStream: stream,
			})
			require.NoError(t, err)
			require.Len(t, result.Removed, 1)

			// DeletePendingObject soft-deletes by setting expires_at.
			require.True(t, transitionDeleteExpiresAtSet(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionDeleteSeedPending(ctx, t, secondary, projectID, "s-only")

			result, err := db.DeletePendingObject(ctx, metabase.DeletePendingObject{
				ObjectStream: stream,
			})
			require.NoError(t, err)
			require.Len(t, result.Removed, 1)

			require.True(t, transitionDeleteExpiresAtSet(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
		})

		t.Run("both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionDeleteSeedPendingBoth(ctx, t, primary, secondary, projectID, "both")

			result, err := db.DeletePendingObject(ctx, metabase.DeletePendingObject{
				ObjectStream: stream,
			})
			require.NoError(t, err)
			// merged result: one removed object from each backend.
			require.Len(t, result.Removed, 2)

			require.True(t, transitionDeleteExpiresAtSet(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
			require.True(t, transitionDeleteExpiresAtSet(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
		})

		t.Run("neither", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := metabase.ObjectStream{
				ProjectID:  projectID,
				BucketName: "bucket",
				ObjectKey:  "missing",
				Version:    1,
				StreamID:   testrand.UUID(),
			}
			_, err := db.DeletePendingObject(ctx, metabase.DeletePendingObject{
				ObjectStream: stream,
			})
			require.True(t, metabase.ErrObjectNotFound.Has(err))
		})
	})
}

func TestTransitionDelete_DeleteObjectLastCommittedPlain(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)

		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")

			result, err := adapter.DeleteObjectLastCommittedPlain(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
			require.Len(t, result.Removed, 1)
			require.Equal(t, stream.StreamID, result.Removed[0].StreamID)

			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")

			result, err := adapter.DeleteObjectLastCommittedPlain(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
			require.Len(t, result.Removed, 1)
			require.Equal(t, stream.StreamID, result.Removed[0].StreamID)

			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
		})

		t.Run("both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionDeleteSeedCommittedBoth(ctx, t, primary, secondary, projectID, "both")

			result, err := adapter.DeleteObjectLastCommittedPlain(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: stream.Location(),
			})
			require.NoError(t, err)
			require.Len(t, result.Removed, 2)

			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), stream.StreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), stream.StreamID))
		})

		t.Run("neither", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			loc := metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: "bucket",
				ObjectKey:  "missing",
			}
			// plain delete reports no error when the object does not exist.
			result, err := adapter.DeleteObjectLastCommittedPlain(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: loc,
			})
			require.NoError(t, err)
			require.Empty(t, result.Removed)
		})
	})
}

func TestTransitionDelete_DeleteObjectLastCommittedVersioned(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		adapter := db.ChooseAdapter(projectID)

		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionSeedCommitted(ctx, t, primary, projectID, "p-only")
			markerStreamID := testrand.UUID()

			result, err := adapter.DeleteObjectLastCommittedVersioned(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: stream.Location(),
				Versioned:      true,
			}, markerStreamID)
			require.NoError(t, err)
			require.Len(t, result.Markers, 1)

			// a delete marker is inserted into the backend owning the object.
			require.True(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), markerStreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), markerStreamID))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionSeedCommitted(ctx, t, secondary, projectID, "s-only")
			markerStreamID := testrand.UUID()

			result, err := adapter.DeleteObjectLastCommittedVersioned(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: stream.Location(),
				Versioned:      true,
			}, markerStreamID)
			require.NoError(t, err)
			require.Len(t, result.Markers, 1)

			// a delete marker is inserted into the backend owning the object.
			require.True(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), markerStreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), markerStreamID))
		})

		t.Run("both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			stream := transitionDeleteSeedCommittedBoth(ctx, t, primary, secondary, projectID, "both")
			markerStreamID := testrand.UUID()

			result, err := adapter.DeleteObjectLastCommittedVersioned(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: stream.Location(),
				Versioned:      true,
			}, markerStreamID)
			require.NoError(t, err)
			// versioned delete is a write routed to home(K): primary wins when
			// the object lives in both, so the marker lands only in primary.
			require.Len(t, result.Markers, 1)

			require.True(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), markerStreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), markerStreamID))
		})

		t.Run("neither", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			loc := metabase.ObjectLocation{
				ProjectID:  projectID,
				BucketName: "bucket",
				ObjectKey:  "missing",
			}
			markerStreamID := testrand.UUID()
			// with no committed object in either backend, a versioned delete of
			// a non-existing object still produces a delete marker.
			result, err := adapter.DeleteObjectLastCommittedVersioned(ctx, metabase.DeleteObjectLastCommitted{
				ObjectLocation: loc,
				Versioned:      true,
			}, markerStreamID)
			require.NoError(t, err)
			// new location → home is primary, so a single marker is produced there.
			require.Len(t, result.Markers, 1)
			require.True(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, primary), markerStreamID))
			require.False(t, transitionContainsStream(transitionDeleteObjectsInAdapter(ctx, t, secondary), markerStreamID))
		})
	})
}

func TestTransitionDelete_DeleteAllBucketObjects(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		bucket := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: "bucket",
		}

		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "p1")
			transitionSeedCommitted(ctx, t, primary, projectID, "p2")

			deleted, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			require.EqualValues(t, 2, deleted)

			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, primary))
			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, secondary))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, secondary, projectID, "s1")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s2")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s3")

			deleted, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			require.EqualValues(t, 3, deleted)

			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, primary))
			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, secondary))
		})

		t.Run("split across both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "p1")
			transitionSeedCommitted(ctx, t, primary, projectID, "p2")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s1")

			deleted, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			// counts summed across both backends.
			require.EqualValues(t, 3, deleted)

			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, primary))
			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, secondary))
		})

		t.Run("empty in both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			deleted, err := db.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			require.EqualValues(t, 0, deleted)
		})
	})
}

func TestTransitionDelete_UncoordinatedDeleteAllBucketObjects(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		bucket := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: "bucket",
		}

		t.Run("primary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "p1")
			transitionSeedCommitted(ctx, t, primary, projectID, "p2")

			deleted, err := db.UncoordinatedDeleteAllBucketObjects(ctx, metabase.UncoordinatedDeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			require.EqualValues(t, 2, deleted)

			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, primary))
			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, secondary))
		})

		t.Run("secondary only", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, secondary, projectID, "s1")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s2")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s3")

			deleted, err := db.UncoordinatedDeleteAllBucketObjects(ctx, metabase.UncoordinatedDeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			require.EqualValues(t, 3, deleted)

			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, primary))
			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, secondary))
		})

		t.Run("split across both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			transitionSeedCommitted(ctx, t, primary, projectID, "p1")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s1")
			transitionSeedCommitted(ctx, t, secondary, projectID, "s2")

			deleted, err := db.UncoordinatedDeleteAllBucketObjects(ctx, metabase.UncoordinatedDeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			// counts summed across both backends.
			require.EqualValues(t, 3, deleted)

			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, primary))
			require.Empty(t, transitionDeleteObjectsInAdapter(ctx, t, secondary))
		})

		t.Run("empty in both", func(t *testing.T) {
			defer metabasetest.DeleteAll{}.Check(ctx, t, db)

			deleted, err := db.UncoordinatedDeleteAllBucketObjects(ctx, metabase.UncoordinatedDeleteAllBucketObjects{
				Bucket: bucket,
			})
			require.NoError(t, err)
			require.EqualValues(t, 0, deleted)
		})
	})
}
