// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

var (
	actions = []pb.PieceAction{
		pb.PieceAction_INVALID,

		pb.PieceAction_PUT,
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_DELETE,

		pb.PieceAction_PUT,
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_DELETE,
	}
)

func TestDB(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		bandwidthdb := db.Bandwidth()

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID
		satellite1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion()).ID

		now := time.Now()

		// ensure zero queries work
		usage, err := bandwidthdb.Summary(ctx, now, now)
		require.NoError(t, err)
		require.Equal(t, &bandwidth.Usage{}, usage)

		usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, now, now)
		require.NoError(t, err)
		require.Equal(t, map[storj.NodeID]*bandwidth.Usage{}, usageBySatellite)

		expectedUsage := &bandwidth.Usage{}
		expectedUsageTotal := &bandwidth.Usage{}

		// add bandwidth usages
		for _, action := range actions {
			expectedUsage.Include(action, int64(action))
			expectedUsageTotal.Include(action, int64(2*action))

			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), now)
			require.NoError(t, err)

			err = bandwidthdb.Add(ctx, satellite1, action, int64(action), now.Add(2*time.Hour))
			require.NoError(t, err)
		}

		// test summarizing
		usage, err = bandwidthdb.Summary(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
		require.NoError(t, err)
		require.Equal(t, expectedUsageTotal, usage)

		expectedUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
			satellite0: expectedUsage,
			satellite1: expectedUsage,
		}
		usageBySatellite, err = bandwidthdb.SummaryBySatellite(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
		require.NoError(t, err)
		require.Equal(t, expectedUsageBySatellite, usageBySatellite)

		// only range capturing second satellite
		usage, err = bandwidthdb.Summary(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
		require.NoError(t, err)
		require.Equal(t, expectedUsage, usage)

		cachedBandwidthUsage, err := bandwidthdb.MonthSummary(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedUsageTotal.Total(), cachedBandwidthUsage)

		// only range capturing second satellite
		expectedUsageBySatellite = map[storj.NodeID]*bandwidth.Usage{
			satellite1: expectedUsage,
		}
		usageBySatellite, err = bandwidthdb.SummaryBySatellite(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
		require.NoError(t, err)
		require.Equal(t, expectedUsageBySatellite, usageBySatellite)
	})
}

func TestCachedBandwidthMonthRollover(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		bandwidthdb := db.Bandwidth()

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID

		y, m, _ := time.Now().Date()
		// Last second of the previous month
		previousMonth := time.Date(y, m, 0, 23, 59, 59, 0, time.Now().UTC().Location())

		// Add data for the previous month.
		for _, action := range actions {
			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), previousMonth)
			require.NoError(t, err)
		}

		cached, err := bandwidthdb.MonthSummary(ctx)
		require.NoError(t, err)
		// Cached bandwidth for this month should still be 0 since CachedBandwidthUsed only looks up by the current month
		require.Equal(t, int64(0), cached)

		thisMonth := previousMonth.Add(time.Second + 1)

		var totalAmount int64
		for _, action := range actions {
			totalAmount += int64(action)
			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), thisMonth)
			require.NoError(t, err)
		}
		cached, err = bandwidthdb.MonthSummary(ctx)
		require.NoError(t, err)
		require.Equal(t, totalAmount, cached)
	})
}

func TestBandwidthRollup(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		err := db.CreateTables()
		if err != nil {
			t.Fatal(err)
		}
		testID1 := storj.NodeID{1}
		testID2 := storj.NodeID{2}
		testID3 := storj.NodeID{3}

		now := time.Now()

		// Create data for 48 hours ago
		err = db.Bandwidth().Add(ctx, testID1, pb.PieceAction_PUT, 1, now.Add(time.Hour*-48))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID1, pb.PieceAction_GET, 2, now.Add(time.Hour*-48))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID1, pb.PieceAction_GET_AUDIT, 3, now.Add(time.Hour*-48))
		require.NoError(t, err)

		// Create data for an hour ago so we can rollup
		err = db.Bandwidth().Add(ctx, testID1, pb.PieceAction_PUT, 2, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID1, pb.PieceAction_GET, 3, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID1, pb.PieceAction_GET_AUDIT, 4, now.Add(time.Hour*-2))
		require.NoError(t, err)

		err = db.Bandwidth().Add(ctx, testID2, pb.PieceAction_PUT, 5, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID2, pb.PieceAction_GET, 6, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID2, pb.PieceAction_GET_AUDIT, 7, now.Add(time.Hour*-2))
		require.NoError(t, err)

		// Test for the data 48 hrs old
		usage, err := db.Bandwidth().Summary(ctx, now.Add(time.Hour*-49), now.Add(time.Hour*-24))
		require.NoError(t, err)
		require.Equal(t, int64(6), usage.Total())

		usage, err = db.Bandwidth().Summary(ctx, now.Add(time.Hour*-24), now)
		require.NoError(t, err)
		require.Equal(t, int64(27), usage.Total())

		err = db.Bandwidth().Rollup(ctx)
		require.NoError(t, err)

		// Test for the 48 hrs ago data again
		usage, err = db.Bandwidth().Summary(ctx, now.Add(time.Hour*-49), now.Add(time.Hour*-24))
		require.NoError(t, err)
		require.Equal(t, int64(6), usage.Total())

		// After rollup, the totals should still be the same
		usage, err = db.Bandwidth().Summary(ctx, now.Add(time.Hour*-24), now)
		require.NoError(t, err)
		require.Equal(t, int64(27), usage.Total())

		// add some data that has already been rolled up to test the date range in the rollup select
		err = db.Bandwidth().Add(ctx, testID2, pb.PieceAction_PUT, 5, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID2, pb.PieceAction_GET, 6, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID2, pb.PieceAction_GET_AUDIT, 7, now.Add(time.Hour*-2))
		require.NoError(t, err)

		// Rollup again
		err = db.Bandwidth().Rollup(ctx)
		require.NoError(t, err)

		// Make sure get the same results as above
		usage, err = db.Bandwidth().Summary(ctx, now.Add(time.Hour*-24), now)
		require.NoError(t, err)
		require.Equal(t, int64(45), usage.Total())

		// Add more data to test the Summary calculates the bandwidth across both tables.
		err = db.Bandwidth().Add(ctx, testID3, pb.PieceAction_PUT, 8, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID3, pb.PieceAction_GET, 9, now.Add(time.Hour*-2))
		require.NoError(t, err)
		err = db.Bandwidth().Add(ctx, testID3, pb.PieceAction_GET_AUDIT, 10, now.Add(time.Hour*-2))
		require.NoError(t, err)

		usage, err = db.Bandwidth().Summary(ctx, now.Add(time.Hour*-24), now)
		require.NoError(t, err)
		require.Equal(t, int64(72), usage.Total())

		usageBySatellite, err := db.Bandwidth().SummaryBySatellite(ctx, now.Add(time.Hour*-49), now)
		require.NoError(t, err)
		for k := range usageBySatellite {
			switch k {
			case testID1:
				require.Equal(t, int64(15), usageBySatellite[testID1].Total())
			case testID2:
				require.Equal(t, int64(36), usageBySatellite[testID2].Total())
			case testID3:
				require.Equal(t, int64(27), usageBySatellite[testID3].Total())
			default:
				require.Fail(t, "Found satellite usage when that shouldn't be there.")
			}
		}
	})
}

func TestDB_Trivial(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		{ // Ensure Add works at all
			err := db.Bandwidth().Add(ctx, testrand.NodeID(), pb.PieceAction_GET, 0, time.Now())
			require.NoError(t, err)
		}

		{ // Ensure MonthSummary works at all
			_, err := db.Bandwidth().MonthSummary(ctx)
			require.NoError(t, err)
		}

		{ // Ensure Summary works at all
			_, err := db.Bandwidth().Summary(ctx, time.Now(), time.Now())
			require.NoError(t, err)
		}

		{ // Ensure SummaryBySatellite works at all
			_, err := db.Bandwidth().SummaryBySatellite(ctx, time.Now(), time.Now())
			require.NoError(t, err)
		}
	})
}
