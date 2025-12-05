// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/date"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestCache(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		satelliteID := testrand.NodeID()

		cache := bandwidth.NewCache(db.Bandwidth())

		expectedUsages := make(map[time.Time]*bandwidth.Usage)
		totalUsages := &bandwidth.Usage{}
		addCache := func(action pb.PieceAction, amount int64, created time.Time) {
			require.NoError(t, cache.Add(ctx, satelliteID, action, amount, created))
			day, _ := date.DayBoundary(created)
			usage := expectedUsages[day]
			if usage == nil {
				usage = &bandwidth.Usage{}
				expectedUsages[day] = usage
			}
			usage.Include(action, amount)
			totalUsages.Include(action, amount)
		}

		day1 := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
		// -- day1
		addCache(pb.PieceAction_PUT, 2, day1.Add(time.Hour))
		addCache(pb.PieceAction_GET, 3, day1.Add(time.Hour*2))
		addCache(pb.PieceAction_GET_AUDIT, 4, day1.Add(time.Hour*3))
		addCache(pb.PieceAction_PUT_REPAIR, 2, day1.Add(time.Hour*4))
		// -- day2
		day2 := day1.AddDate(0, 0, 1)
		addCache(pb.PieceAction_GET, 3, day2.Add(time.Hour))

		// check that the bandwidth cache has the expected values
		{
			usage, err := cache.Summary(ctx, time.Time{}, day1.Add(-24*time.Hour))
			require.NoError(t, err)
			require.Equal(t, int64(0), usage.Total())

			usage, err = cache.Summary(ctx, time.Time{}, day1)
			require.NoError(t, err)
			require.Equal(t, expectedUsages[day1].Total(), usage.Total())

			usage, err = cache.Summary(ctx, time.Time{}, time.Now())
			require.NoError(t, err)
			require.Equal(t, expectedUsages[day1].Total()+expectedUsages[day2].Total(), usage.Total())
		}

		// Let's persist the cache to the database
		require.NoError(t, cache.Persist(ctx))

		// add more data to the cache
		addCache(pb.PieceAction_GET, 3, day2.Add(time.Hour*5))
		addCache(pb.PieceAction_GET_REPAIR, 3, day2.Add(time.Hour*6))
		addCache(pb.PieceAction_PUT, 4, day2.Add(time.Hour*7))

		{
			usage, err := cache.Summary(ctx, day1, time.Now())
			require.NoError(t, err)
			require.Equal(t, expectedUsages[day1].Total()+expectedUsages[day2].Total(), usage.Total())

			monthSummary, err := cache.MonthSummary(ctx, day2.Add(24*time.Hour))
			require.NoError(t, err)
			require.Equal(t, expectedUsages[day1].Total()+expectedUsages[day2].Total(), monthSummary)

			expectedDailyRollups := make([]bandwidth.UsageRollup, len(expectedUsages))
			num := 0
			for day, usage := range expectedUsages {
				rollup := usage.Rollup(day)
				expectedDailyRollups[num] = *rollup
				num++
			}
			sort.SliceStable(expectedDailyRollups, func(i, j int) bool {
				return expectedDailyRollups[i].IntervalStart.Before(expectedDailyRollups[j].IntervalStart)
			})
			dailyRollup, err := cache.GetDailyRollups(ctx, day1, time.Now())
			require.NoError(t, err)
			require.Len(t, dailyRollup, len(expectedDailyRollups))
			require.Equal(t, expectedDailyRollups, dailyRollup)

			dailyRollup, err = cache.GetDailySatelliteRollups(ctx, satelliteID, day1, time.Now())
			require.NoError(t, err)
			require.Len(t, dailyRollup, len(expectedDailyRollups))
			require.Equal(t, expectedDailyRollups, dailyRollup)

			// no data for this satellite
			dailyRollup, err = cache.GetDailySatelliteRollups(ctx, testrand.NodeID(), day1, time.Now())
			require.NoError(t, err)
			require.Len(t, dailyRollup, 0)
		}
	})
}
