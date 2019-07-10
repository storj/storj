// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
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
