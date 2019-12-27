// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storageusage_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/storageusage"
)

func TestStorageUsage(t *testing.T) {
	const (
		satelliteNum = 10
		days         = 30
	)

	now := time.Now()

	var satellites []storj.NodeID
	satelliteID := testrand.NodeID()

	satellites = append(satellites, satelliteID)
	for i := 0; i < satelliteNum-1; i++ {
		satellites = append(satellites, testrand.NodeID())
	}

	stamps, summary := makeStorageUsageStamps(satellites, days, now)

	var totalSummary float64
	for _, summ := range summary {
		totalSummary += summ
	}

	dailyStamps := make(map[time.Time]storageusage.Stamp)
	dailyStampsTotals := make(map[time.Time]float64)

	for _, stamp := range stamps {
		if stamp.SatelliteID == satelliteID {
			dailyStamps[stamp.IntervalStart.UTC()] = stamp
		}

		dailyStampsTotals[stamp.IntervalStart.UTC()] += stamp.AtRestTotal
	}

	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		storageUsageDB := db.StorageUsage()

		t.Run("test store", func(t *testing.T) {
			err := storageUsageDB.Store(ctx, stamps)
			assert.NoError(t, err)
		})

		t.Run("test get daily", func(t *testing.T) {
			res, err := storageUsageDB.GetDaily(ctx, satelliteID, time.Time{}, now)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, days, len(res))

			for _, stamp := range res {
				assert.Equal(t, satelliteID, stamp.SatelliteID)
				assert.Equal(t, dailyStamps[stamp.IntervalStart].AtRestTotal, stamp.AtRestTotal)
			}
		})

		t.Run("test get daily total", func(t *testing.T) {
			res, err := storageUsageDB.GetDailyTotal(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, days, len(res))

			for _, stamp := range res {
				assert.Equal(t, dailyStampsTotals[stamp.IntervalStart], stamp.AtRestTotal)
			}
		})

		t.Run("test summary satellite", func(t *testing.T) {
			summ, err := storageUsageDB.SatelliteSummary(ctx, satelliteID, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, summary[satelliteID], summ)
		})

		t.Run("test summary", func(t *testing.T) {
			summ, err := storageUsageDB.Summary(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, totalSummary, summ)
		})
	})
}

func TestEmptyStorageUsage(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var emptySummary float64
		now := time.Now()

		storageUsageDB := db.StorageUsage()

		t.Run("test get daily", func(t *testing.T) {
			res, err := storageUsageDB.GetDaily(ctx, storj.NodeID{}, time.Time{}, now)
			assert.NoError(t, err)
			assert.Nil(t, res)
		})

		t.Run("test get daily total", func(t *testing.T) {
			res, err := storageUsageDB.GetDailyTotal(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.Nil(t, res)
		})

		t.Run("test summary satellite", func(t *testing.T) {
			summ, err := storageUsageDB.SatelliteSummary(ctx, storj.NodeID{}, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, emptySummary, summ)
		})

		t.Run("test summary", func(t *testing.T) {
			summ, err := storageUsageDB.Summary(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, emptySummary, summ)
		})
	})
}

// makeStorageUsageStamps creates storage usage stamps and expected summaries for provided satellites.
// Creates one entry per day for 30 days with last date as beginning of provided endDate.
func makeStorageUsageStamps(satellites []storj.NodeID, days int, endDate time.Time) ([]storageusage.Stamp, map[storj.NodeID]float64) {
	var stamps []storageusage.Stamp
	summary := make(map[storj.NodeID]float64)

	startDate := time.Date(endDate.Year(), endDate.Month(), endDate.Day()-days, 0, 0, 0, 0, endDate.Location())

	for _, satellite := range satellites {
		for i := 0; i < days; i++ {
			stamp := storageusage.Stamp{
				SatelliteID:   satellite,
				AtRestTotal:   math.Round(testrand.Float64n(1000)),
				IntervalStart: startDate.Add(time.Hour * 24 * time.Duration(i)),
			}

			summary[satellite] += stamp.AtRestTotal
			stamps = append(stamps, stamp)
		}
	}

	return stamps, summary
}
