// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

// transitionSeedCommitted inserts a committed unversioned object directly into
// the given backend, bypassing the transition routing.
func transitionSeedCommitted(ctx *testcontext.Context, t *testing.T, adapter metabase.Adapter, projectID uuid.UUID, key string) metabase.ObjectStream {
	stream := metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: "bucket",
		ObjectKey:  metabase.ObjectKey(key),
		Version:    1,
		StreamID:   testrand.UUID(),
	}
	require.NoError(t, adapter.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
		ObjectStream: stream,
		Status:       metabase.CommittedUnversioned,
		Encryption:   metabasetest.DefaultEncryption,
	}}))
	return stream
}

func transitionContainsStream(objects []metabase.RawObject, streamID uuid.UUID) bool {
	for _, o := range objects {
		if o.StreamID == streamID {
			return true
		}
	}
	return false
}

func TestTransition_Routing(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// the configured project routes through the transition adapter.
		require.True(t, strings.HasPrefix(db.ChooseAdapter(projectID).Name(), "transition("))
		// any other project routes straight to the primary adapter.
		require.Equal(t, primary.Name(), db.ChooseAdapter(testrand.UUID()).Name())
	})
}

func TestTransition_ReadFallback(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// object only exists in the secondary (old) backend.
		stream := transitionSeedCommitted(ctx, t, secondary, projectID, "only-in-secondary")

		// a read through the transition adapter misses primary and falls back.
		obj, err := db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
			ObjectLocation: stream.Location(),
		})
		require.NoError(t, err)
		require.Equal(t, stream.StreamID, obj.StreamID)
	})
}

func TestTransition_WriteGoesToPrimary(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		stream := metabase.ObjectStream{
			ProjectID:  projectID,
			BucketName: "bucket",
			ObjectKey:  "new-key",
			Version:    1,
			StreamID:   testrand.UUID(),
		}
		// a brand-new object created through the transition adapter lands in primary.
		metabasetest.CreateObject(ctx, t, db, stream, 0)

		_, err := primary.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
			ObjectLocation: stream.Location(),
			Version:        stream.Version,
		})
		require.NoError(t, err)

		_, err = secondary.GetObjectExactVersion(ctx, metabase.GetObjectExactVersion{
			ObjectLocation: stream.Location(),
			Version:        stream.Version,
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))
	})
}

func TestTransition_BeginCoLocatesWithSecondary(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// an existing committed object for this location lives in secondary.
		transitionSeedCommitted(ctx, t, secondary, projectID, "existing")

		// beginning a new upload at the same location must co-locate in secondary.
		begin := metabase.ObjectStream{
			ProjectID:  projectID,
			BucketName: "bucket",
			ObjectKey:  "existing",
			Version:    2,
			StreamID:   testrand.UUID(),
		}
		_, err := db.BeginObjectExactVersion(ctx, metabase.BeginObjectExactVersion{
			ObjectStream: begin,
			Encryption:   metabasetest.DefaultEncryption,
		})
		require.NoError(t, err)

		inSecondary, err := secondary.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.True(t, transitionContainsStream(inSecondary, begin.StreamID))

		inPrimary, err := primary.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.False(t, transitionContainsStream(inPrimary, begin.StreamID))
	})
}

func TestTransition_DeleteRemovesFromBothBackends(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// simulate a relocation window: the same object exists in both backends.
		stream := transitionSeedCommitted(ctx, t, primary, projectID, "duplicated")
		require.NoError(t, secondary.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
			ObjectStream: stream,
			Status:       metabase.CommittedUnversioned,
			Encryption:   metabasetest.DefaultEncryption,
		}}))

		_, err := db.DeleteObjectExactVersion(ctx, metabase.DeleteObjectExactVersion{
			ObjectLocation: stream.Location(),
			Version:        stream.Version,
		})
		require.NoError(t, err)

		inPrimary, err := primary.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.False(t, transitionContainsStream(inPrimary, stream.StreamID))

		inSecondary, err := secondary.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.False(t, transitionContainsStream(inSecondary, stream.StreamID))
	})
}

func TestTransition_ListMergesBothBackends(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// one object created through the transition adapter (lands in primary),
		// one seeded directly into secondary.
		primaryStream := metabase.ObjectStream{
			ProjectID:  projectID,
			BucketName: "bucket",
			ObjectKey:  "a-primary",
			Version:    1,
			StreamID:   testrand.UUID(),
		}
		metabasetest.CreateObject(ctx, t, db, primaryStream, 0)
		transitionSeedCommitted(ctx, t, secondary, projectID, "b-secondary")

		result, err := db.ListObjects(ctx, metabase.ListObjects{
			ProjectID:  projectID,
			BucketName: "bucket",
			Recursive:  true,
			Limit:      10,
		})
		require.NoError(t, err)

		var keys []string
		for _, o := range result.Objects {
			keys = append(keys, string(o.ObjectKey))
		}
		require.ElementsMatch(t, []string{"a-primary", "b-secondary"}, keys)
	})
}

func TestTransition_WithTxFailsClosed(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		called := false
		err := db.ChooseAdapter(projectID).WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, tx metabase.TransactionAdapter) error {
			called = true
			return nil
		})
		require.Error(t, err)
		require.False(t, called, "transaction body must not run on the transition adapter")
	})
}
