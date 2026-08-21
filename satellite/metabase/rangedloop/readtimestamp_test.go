// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/shared/dbutil"
)

// Bloom filters built from a scan that is not a snapshot can omit live pieces,
// so a run has to read at a fixed timestamp, one the caller pinned or one
// derived from the stale interval, on a backend that can serve it.
func TestMetabaseRangeSplitterReadsSnapshot(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		split := func(cfg rangedloop.Config, readTimestamp time.Time) bool {
			return rangedloop.NewMetabaseRangeSplitterWithReadTimestamp(zaptest.NewLogger(t), db, cfg, readTimestamp).ReadsSnapshot()
		}
		serves := rangedloop.ServesFixedReadTimestamp(db.Implementations())

		require.False(t, split(rangedloop.Config{}, time.Time{}))

		// a stale interval gives every pass a fixed timestamp, and so does a
		// timestamp the caller has already pinned, where the backend serves it
		require.Equal(t, serves, split(rangedloop.Config{StaleInterval: 5 * time.Minute}, time.Time{}))
		require.Equal(t, serves, split(rangedloop.Config{}, time.Now()))

		// a metabase whose backend cannot serve one may be read live for
		// testing, and then needs no fixed timestamp
		require.True(t, split(rangedloop.Config{StaleInterval: 5 * time.Minute, AllowLiveReads: true}, time.Time{}))
		require.Equal(t, !serves, split(rangedloop.Config{AllowLiveReads: true}, time.Time{}))
	})
}

// The counts of a validated run have to come from the snapshot the scan read,
// which a live-reading backend cannot serve, so the validation is left out
// there instead of comparing live counts against the scan.
func TestSegmentsCountValidationLeavesOutLiveBackend(t *testing.T) {
	metabasetest.Run(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB) {
		// From the database clock once the schema exists, as a read at a fixed
		// timestamp sees the schema of that time too; then let a moment pass,
		// as TiDB and CockroachDB refuse timestamps at or after the current time.
		checkTimestamp, err := db.Now(ctx)
		require.NoError(t, err)
		time.Sleep(500 * time.Millisecond)

		validation := rangedloop.NewSegmentsCountValidation(zaptest.NewLogger(t), db, checkTimestamp, 0)
		require.NoError(t, validation.Start(ctx, checkTimestamp))
		require.NoError(t, validation.Finish(ctx))

		// and without a timestamp the counts would describe no snapshot at all
		validation = rangedloop.NewSegmentsCountValidation(zaptest.NewLogger(t), db, time.Time{}, 0)
		require.NoError(t, validation.Start(ctx, checkTimestamp))
		require.NoError(t, validation.Finish(ctx))
	})
}

// The counts of a validated run have to come from the snapshot the scan read,
// which a live-reading backend cannot serve.
func TestServesFixedReadTimestamp(t *testing.T) {
	require.False(t, rangedloop.ServesFixedReadTimestamp(map[string]dbutil.Implementation{"p": dbutil.Postgres}))
	require.False(t, rangedloop.ServesFixedReadTimestamp(map[string]dbutil.Implementation{"p": dbutil.Postgres, "c": dbutil.Cockroach}))
	require.True(t, rangedloop.ServesFixedReadTimestamp(map[string]dbutil.Implementation{"t": dbutil.TiDB}))
}

// Live reads only qualify as a snapshot on a metabase with no backend that
// could serve the timestamp: a mixed one would read a partial snapshot.
func TestCanReadSnapshot(t *testing.T) {
	postgres := map[string]dbutil.Implementation{"p": dbutil.Postgres}
	mixed := map[string]dbutil.Implementation{"p": dbutil.Postgres, "t": dbutil.TiDB}
	tidb := map[string]dbutil.Implementation{"t": dbutil.TiDB}

	fixed, live := true, false

	// a fixed timestamp is a snapshot where every backend serves it
	require.True(t, rangedloop.CanReadSnapshot(fixed, false, tidb))
	require.False(t, rangedloop.CanReadSnapshot(fixed, false, postgres))
	require.False(t, rangedloop.CanReadSnapshot(fixed, false, mixed))

	// live reads pass only where no backend could have served a timestamp
	require.True(t, rangedloop.CanReadSnapshot(live, true, postgres))
	require.True(t, rangedloop.CanReadSnapshot(fixed, true, postgres))
	require.False(t, rangedloop.CanReadSnapshot(live, true, tidb))
	require.False(t, rangedloop.CanReadSnapshot(fixed, true, mixed))
	require.False(t, rangedloop.CanReadSnapshot(live, true, mixed))

	require.False(t, rangedloop.CanReadSnapshot(live, false, postgres))

	// a metabase without backends is no snapshot of anything
	require.False(t, rangedloop.CanReadSnapshot(fixed, false, nil))
	require.False(t, rangedloop.CanReadSnapshot(live, true, nil))
}
