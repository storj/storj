// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
)

func TestMirror_Routing(t *testing.T) {
	metabasetest.RunMirror(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// the configured project routes through the mirror adapter.
		require.True(t, strings.HasPrefix(db.ChooseAdapter(projectID).Name(), "mirror("))
		// any other project routes straight to the primary adapter.
		require.Equal(t, primary.Name(), db.ChooseAdapter(testrand.UUID()).Name())
	})
}

func TestMirror_ReadsServedByPrimaryOnly(t *testing.T) {
	metabasetest.RunMirror(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		// object only exists in the secondary backend.
		stream := transitionSeedCommitted(ctx, t, secondary, projectID, "only-in-secondary")

		// the mirror adapter never falls back to secondary for reads.
		_, err := db.GetObjectLastCommitted(ctx, metabase.GetObjectLastCommitted{
			ObjectLocation: stream.Location(),
		})
		require.True(t, metabase.ErrObjectNotFound.Has(err))
	})
}

func TestMirror_WriteMirroredToSecondary(t *testing.T) {
	metabasetest.RunMirror(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		stream := metabase.ObjectStream{
			ProjectID:  projectID,
			BucketName: "bucket",
			ObjectKey:  "mirrored",
			Version:    1,
			StreamID:   testrand.UUID(),
		}
		metabasetest.CreateObject(ctx, t, db, stream, 0)

		// primary is authoritative and has the object synchronously.
		inPrimary, err := primary.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.True(t, transitionContainsStream(inPrimary, stream.StreamID))

		// the secondary receives the write in the background.
		require.Eventually(t, func() bool {
			inSecondary, err := secondary.TestingGetAllObjects(ctx)
			require.NoError(t, err)
			return transitionContainsStream(inSecondary, stream.StreamID)
		}, 10*time.Second, 50*time.Millisecond)
	})
}
