// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
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

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID
		satellite1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion()).ID

		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)

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

		expectedUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
			satellite0: expectedUsage,
			satellite1: expectedUsage,
		}

		// test summarizing
		{
			usage, err := bandwidthdb.Summary(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedUsageTotal, usage)

			// only range capturing second satellite
			usage, err = bandwidthdb.Summary(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedUsage, usage)
		}

		{
			usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite, usageBySatellite)

			// only range capturing second satellite
			expectedUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
				satellite1: expectedUsage,
			}

			usageBySatellite, err = bandwidthdb.SummaryBySatellite(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite, usageBySatellite)
		}

		{
			usage, err := bandwidthdb.SatelliteSummary(ctx, satellite0, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite[satellite0], usage)

			usage, err = bandwidthdb.SatelliteSummary(ctx, satellite1, time.Time{}, now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite[satellite1], usage)
		}

		{
			cachedBandwidthUsage, err := bandwidthdb.MonthSummary(ctx, now)
			require.NoError(t, err)
			require.Equal(t, expectedUsageTotal.Total(), cachedBandwidthUsage)
		}
	})
}

func TestEgressSummary(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID
		satellite1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion()).ID

		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)

		expectedEgressUsage := &bandwidth.Usage{}
		expectedEgressUsageTotal := &bandwidth.Usage{}

		// add egress usages.
		for _, action := range egressActions {
			expectedEgressUsage.Include(action, int64(action))
			expectedEgressUsageTotal.Include(action, int64(2*action))

			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), now)
			require.NoError(t, err)

			err = bandwidthdb.Add(ctx, satellite1, action, int64(action), now.Add(2*time.Hour))
			require.NoError(t, err)
		}

		expectedEgressUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
			satellite0: expectedEgressUsage,
			satellite1: expectedEgressUsage,
		}

		// test egress summarizing.
		{
			usage, err := bandwidthdb.EgressSummary(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageTotal, usage)

			// only range capturing second satellite.
			usage, err = bandwidthdb.EgressSummary(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsage, usage)
		}

		{
			usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageBySatellite, usageBySatellite)

			// only range capturing second satellite.
			expectedEgressUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
				satellite1: expectedEgressUsage,
			}

			usageBySatellite, err = bandwidthdb.SummaryBySatellite(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageBySatellite, usageBySatellite)
		}

		{
			usage, err := bandwidthdb.SatelliteEgressSummary(ctx, satellite0, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageBySatellite[satellite0], usage)

			usage, err = bandwidthdb.SatelliteEgressSummary(ctx, satellite1, time.Time{}, now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedEgressUsageBySatellite[satellite1], usage)
		}
	})
}

func TestIngressSummary(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID
		satellite1 := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion()).ID

		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)

		expectedIngressUsage := &bandwidth.Usage{}
		expectedIngressUsageTotal := &bandwidth.Usage{}

		// add ingress usages.
		for _, action := range ingressActions {
			expectedIngressUsage.Include(action, int64(action))
			expectedIngressUsageTotal.Include(action, int64(2*action))

			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), now)
			require.NoError(t, err)

			err = bandwidthdb.Add(ctx, satellite1, action, int64(action), now.Add(2*time.Hour))
			require.NoError(t, err)
		}

		expectedIngressUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
			satellite0: expectedIngressUsage,
			satellite1: expectedIngressUsage,
		}

		// test ingress summarizing.
		{
			usage, err := bandwidthdb.IngressSummary(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsageTotal, usage)

			// only range capturing second satellite.
			usage, err = bandwidthdb.IngressSummary(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsage, usage)
		}

		{
			usageBySatellite, err := bandwidthdb.SummaryBySatellite(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsageBySatellite, usageBySatellite)

			// only range capturing second satellite.
			expectedUsageBySatellite := map[storj.NodeID]*bandwidth.Usage{
				satellite1: expectedIngressUsage,
			}

			usageBySatellite, err = bandwidthdb.SummaryBySatellite(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedUsageBySatellite, usageBySatellite)
		}

		{
			usage, err := bandwidthdb.SatelliteIngressSummary(ctx, satellite0, time.Time{}, now)
			require.NoError(t, err)
			require.Equal(t, expectedIngressUsageBySatellite[satellite0], usage)

			usage, err = bandwidthdb.SatelliteIngressSummary(ctx, satellite1, time.Time{}, now.Add(10*time.Hour))
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

func TestBandwidthDailyRollups(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		const (
			numSatellites = 5
			days          = 30
			hours         = 12
		)

		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)
		startDate := time.Date(now.Year(), now.Month(), now.Day()-days, 0, 0, 0, 0, now.Location())

		bandwidthDB := db.Bandwidth()

		totalUsageRollups := make(map[time.Time]*bandwidth.UsageRollup)

		addBandwidth := func(day time.Time, satellite storj.NodeID, r *bandwidth.UsageRollup) {
			if totalUsageRollups[day] == nil {
				totalUsageRollups[day] = &bandwidth.UsageRollup{
					IntervalStart: day,
				}
			}

			for h := 0; h < hours; h++ {
				get := testrand.Int63n(1000)
				getRepair := testrand.Int63n(1000)
				getAudit := testrand.Int63n(1000)
				put := testrand.Int63n(1000)
				putRepair := testrand.Int63n(1000)
				_delete := testrand.Int63n(1000)

				// add bandwidth
				err := bandwidthDB.Add(ctx, satellite, pb.PieceAction_GET, get, day.Add(time.Hour*time.Duration(h)))
				require.NoError(t, err)
				err = bandwidthDB.Add(ctx, satellite, pb.PieceAction_GET_REPAIR, getRepair, day.Add(time.Hour*time.Duration(h)))
				require.NoError(t, err)
				err = bandwidthDB.Add(ctx, satellite, pb.PieceAction_GET_AUDIT, getAudit, day.Add(time.Hour*time.Duration(h)))
				require.NoError(t, err)
				err = bandwidthDB.Add(ctx, satellite, pb.PieceAction_PUT, put, day.Add(time.Hour*time.Duration(h)))
				require.NoError(t, err)
				err = bandwidthDB.Add(ctx, satellite, pb.PieceAction_PUT_REPAIR, putRepair, day.Add(time.Hour*time.Duration(h)))
				require.NoError(t, err)
				err = bandwidthDB.Add(ctx, satellite, pb.PieceAction_DELETE, _delete, day.Add(time.Hour*time.Duration(h)))
				require.NoError(t, err)

				r.Egress.Usage += get
				r.Egress.Repair += getRepair
				r.Egress.Audit += getAudit
				r.Ingress.Usage += put
				r.Ingress.Repair += putRepair
				r.Delete += _delete

				totalUsageRollups[day].Egress.Usage += get
				totalUsageRollups[day].Egress.Repair += getRepair
				totalUsageRollups[day].Egress.Audit += getAudit
				totalUsageRollups[day].Ingress.Usage += put
				totalUsageRollups[day].Ingress.Repair += putRepair
				totalUsageRollups[day].Delete += _delete
			}
		}

		satelliteID := testrand.NodeID()

		var satellites []storj.NodeID
		satellites = append(satellites, satelliteID)

		for i := 0; i < numSatellites-1; i++ {
			satellites = append(satellites, testrand.NodeID())
		}

		usageRollups := make(map[storj.NodeID]map[time.Time]*bandwidth.UsageRollup)

		for _, satellite := range satellites {
			usageRollups[satellite] = make(map[time.Time]*bandwidth.UsageRollup)

			for d := 0; d < days-1; d++ {
				day := startDate.Add(time.Hour * 24 * time.Duration(d))

				usageRollup := &bandwidth.UsageRollup{
					IntervalStart: day,
				}

				addBandwidth(day, satellite, usageRollup)
				usageRollups[satellite][day] = usageRollup
			}

		}

		// perform rollup for but last day
		err := bandwidthDB.Rollup(ctx)
		require.NoError(t, err)

		// last day add bandwidth that won't be rolled up
		day := startDate.Add(time.Hour * 24 * time.Duration(days-1))

		for _, satellite := range satellites {
			usageRollup := &bandwidth.UsageRollup{
				IntervalStart: day,
			}

			addBandwidth(day, satellite, usageRollup)
			usageRollups[satellite][day] = usageRollup
		}

		{
			rolls, err := bandwidthDB.GetDailySatelliteRollups(ctx, satelliteID, time.Time{}, now)

			assert.NoError(t, err)
			assert.NotNil(t, rolls)
			assert.Equal(t, days, len(rolls))

			for _, rollup := range rolls {
				expected := *usageRollups[satelliteID][rollup.IntervalStart]
				assert.Equal(t, expected, rollup)
			}
		}

		{
			rolls, err := bandwidthDB.GetDailyRollups(ctx, time.Time{}, now)

			assert.NoError(t, err)
			assert.NotNil(t, rolls)
			assert.Equal(t, days, len(rolls))

			for _, rollup := range rolls {
				assert.Equal(t, *totalUsageRollups[rollup.IntervalStart], rollup)
			}
		}
	})
}

func TestCachedBandwidthMonthRollover(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		bandwidthdb := db.Bandwidth()

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID

		// Compute times that will be used for the test
		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)
		previousMonth := time.Date(2010, 3, 31, 23, 59, 59, 0, time.UTC)
		thisMonth := previousMonth.Add(time.Second)

		// Add data for the previous month.
		for _, action := range actions {
			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), previousMonth)
			require.NoError(t, err)
		}

		// Cached bandwidth for this month should still be 0 since CachedBandwidthUsed only looks up by the current month
		cached, err := bandwidthdb.MonthSummary(ctx, now)
		require.NoError(t, err)
		require.Equal(t, int64(0), cached)

		var totalAmount int64
		for _, action := range actions {
			totalAmount += int64(action)
			err := bandwidthdb.Add(ctx, satellite0, action, int64(action), thisMonth)
			require.NoError(t, err)
		}

		cached, err = bandwidthdb.MonthSummary(ctx, now)
		require.NoError(t, err)
		require.Equal(t, totalAmount, cached)
	})
}

func TestBandwidthRollup(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		err := db.MigrateToLatest(ctx)
		if err != nil {
			t.Fatal(err)
		}
		testID1 := storj.NodeID{1}
		testID2 := storj.NodeID{2}
		testID3 := storj.NodeID{3}

		now := time.Date(2010, 4, 7, 12, 30, 00, 0, time.UTC)

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
