// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
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

var (
	egressActions = []pb.PieceAction{
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
	}
)

var (
	ingressActions = []pb.PieceAction{
		pb.PieceAction_PUT,
		pb.PieceAction_PUT_REPAIR,
	}
)

func TestBandwidthDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()

		satellite0 := testrand.NodeID()
		satellite1 := testrand.NodeID()

		now := time.Now()
		past := now.Add(-48 * time.Hour)

		expectedUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
			satellite0: {},
			satellite1: {},
		}

		expectedUsageTotal := &bandwidth.Usage{}

		// add bandwidth usages
		for _, action := range actions {
			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), past)
			require.NoError(t, err)
			expectedUsageBySatellite[satellite0].Include(action, int64(action))

			err = bandwidthdb.Add(ctx, satellite1, action, int64(action), past.Add(2*time.Hour))
			require.NoError(t, err)
			expectedUsageBySatellite[satellite1].Include(action, int64(action))

			expectedUsageTotal.Include(action, int64(2*action))
		}

		// test summarizing
		{
			usage, err := bandwidthdb.Summary(ctx, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedUsageTotal, usage)
		}

		{
			usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite, usageBySatellite)
		}

		{
			usage, err := bandwidthdb.SatelliteSummary(ctx, satellite0, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite[satellite0], usage)

			usage, err = bandwidthdb.SatelliteSummary(ctx, satellite1, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite[satellite1], usage)
		}

		{
			totalUsage, err := bandwidthdb.MonthSummary(ctx, past)
			require.NoError(t, err)

			if now.Month() != past.Month() {
				nowMonth, err := bandwidthdb.MonthSummary(ctx, now)
				require.NoError(t, err)
				totalUsage += nowMonth
			}

			require.Equal(t, expectedUsageTotal.Total(), totalUsage)
		}
	})
}

func TestEgressSummary(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()

		satellite0 := testrand.NodeID()
		satellite1 := testrand.NodeID()

		now := time.Now()
		past := now.Add(-48 * time.Hour)

		expectedEgressUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
			satellite0: {},
			satellite1: {},
		}

		expectedEgressUsageTotal := &bandwidth.Usage{}

		// add egress usages
		for _, action := range egressActions {
			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), past)
			require.NoError(t, err)
			expectedEgressUsageBySatellite[satellite0].Include(action, int64(action))

			err = bandwidthdb.Add(ctx, satellite1, action, int64(action), past.Add(2*time.Hour))
			require.NoError(t, err)
			expectedEgressUsageBySatellite[satellite1].Include(action, int64(action))

			expectedEgressUsageTotal.Include(action, int64(2*action))
		}

		// test egress summarizing.
		{
			// only range capturing second satellite.
			usage, err := bandwidthdb.EgressSummary(ctx, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageTotal, usage)
		}

		{
			usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageBySatellite, usageBySatellite)
		}

		{
			usage, err := bandwidthdb.SatelliteEgressSummary(ctx, satellite0, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageBySatellite[satellite0], usage)

			usage, err = bandwidthdb.SatelliteEgressSummary(ctx, satellite1, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageBySatellite[satellite1], usage)
		}
	})
}

func TestIngressSummary(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()

		satellite0 := testrand.NodeID()
		satellite1 := testrand.NodeID()

		now := time.Now()
		past := now.Add(-48 * time.Hour)

		expectedIngressUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
			satellite0: {},
			satellite1: {},
		}

		expectedIngressUsage := &bandwidth.Usage{}

		// add ingress usages
		for _, action := range ingressActions {
			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), past)
			require.NoError(t, err)
			expectedIngressUsageBySatellite[satellite0].Include(action, int64(action))

			err = bandwidthdb.Add(ctx, satellite1, action, int64(action), past.Add(2*time.Hour))
			require.NoError(t, err)
			expectedIngressUsageBySatellite[satellite1].Include(action, int64(action))

			expectedIngressUsage.Include(action, int64(2*action))
		}

		// test ingress summarizing.
		{
			// only range capturing second satellite.
			usage, err := bandwidthdb.IngressSummary(ctx, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsage, usage)
		}

		{
			usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsageBySatellite, usageBySatellite)
		}

		{
			usage, err := bandwidthdb.SatelliteIngressSummary(ctx, satellite0, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsageBySatellite[satellite0], usage)

			usage, err = bandwidthdb.SatelliteIngressSummary(ctx, satellite1, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsageBySatellite[satellite1], usage)
		}
	})
}

func TestEmptyBandwidthDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()

		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)

		{
			usage, err := bandwidthdb.Summary(ctx, now, now)
			require.NoError(t, err)
			require.Equal(t, &bandwidth.Usage{}, usage)
		}

		{
			usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, now, now)
			require.NoError(t, err)
			require.Equal(t, map[storj.NodeID]*bandwidth.Usage{}, usageBySatellite)
		}

		{
			usage, err := bandwidthdb.SatelliteSummary(ctx, storj.NodeID{}, now, now)
			require.NoError(t, err)
			require.Equal(t, &bandwidth.Usage{}, usage)
		}

		{
			rollups, err := bandwidthdb.GetDailyRollups(ctx, now, now)
			require.NoError(t, err)
			require.Equal(t, []bandwidth.UsageRollup(nil), rollups)
		}

		{
			rollups, err := bandwidthdb.GetDailySatelliteRollups(ctx, storj.NodeID{}, now, now)
			require.NoError(t, err)
			require.Equal(t, []bandwidth.UsageRollup(nil), rollups)
		}
	})
}

func TestBandwidthDBCache(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()
		bandwidthCache := bandwidth.NewCache(bandwidthdb)

		satellite0 := testrand.NodeID()

		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)

		// Add data for this month.
		var totalAmount int64
		for _, action := range actions {
			totalAmount += int64(action)
			err := bandwidthCache.Add(ctx, satellite0, action, int64(action), now)
			require.NoError(t, err)
		}

		// Check that the data is not present in the database (yet).
		summary, err := bandwidthdb.MonthSummary(ctx, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), summary)

		// Persist the cache.
		err = bandwidthCache.Persist(ctx)
		require.NoError(t, err)

		// Check that the data is now present in the database.
		summary, err = bandwidthdb.MonthSummary(ctx, now)
		require.NoError(t, err)
		require.Equal(t, totalAmount, summary)

		// Get the data through the cache.
		summaryCache, err := bandwidthCache.MonthSummary(ctx, now)
		require.NoError(t, err)
		require.Equal(t, summary, summaryCache)
	})
}

func TestDB_Trivial(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		{ // Ensure Add works at all
			err := db.Bandwidth().Add(ctx, testrand.NodeID(), pb.PieceAction_GET, 0, time.Now())
			require.NoError(t, err)
		}

		{ // Ensure MonthSummary works at all
			_, err := db.Bandwidth().MonthSummary(ctx, time.Now())
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
