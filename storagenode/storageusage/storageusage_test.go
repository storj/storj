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

	now := time.Now().UTC()

	var satellites []storj.NodeID
	satelliteID := testrand.NodeID()

	satellites = append(satellites, satelliteID)
	for i := 0; i < satelliteNum-1; i++ {
		satellites = append(satellites, testrand.NodeID())
	}

	stamps := storagenodedbtest.MakeStorageUsageStamps(satellites, days, now)

	var totalSummary float64
	expectedDailyStamps := make(map[storj.NodeID]map[time.Time]storageusage.Stamp)
	expectedDailyStampsTotals := make(map[time.Time]float64)
	summary := make(map[storj.NodeID]float64)

	for _, stamp := range stamps {
		if expectedDailyStamps[stamp.SatelliteID] == nil {
			expectedDailyStamps[stamp.SatelliteID] = map[time.Time]storageusage.Stamp{}
		}
		expectedDailyStamps[stamp.SatelliteID][stamp.IntervalStart.UTC()] = stamp
	}

	for _, satellite := range satellites {
		for _, stamp := range expectedDailyStamps[satellite] {
			intervalStart := stamp.IntervalStart.UTC()
			prevTimestamp := intervalStart.AddDate(0, 0, -1)
			atRestTotal := stamp.AtRestTotal / 24
			if prevStamp, ok := expectedDailyStamps[satellite][prevTimestamp]; ok {
				diff := stamp.IntervalEndTime.UTC().Sub(prevStamp.IntervalEndTime.UTC()).Hours()
				atRestTotal = stamp.AtRestTotal / diff
			}
			expectedDailyStamps[satellite][intervalStart] = storageusage.Stamp{
				SatelliteID:     satellite,
				AtRestTotal:     atRestTotal,
				IntervalStart:   intervalStart,
				IntervalEndTime: stamp.IntervalEndTime,
			}
			expectedDailyStampsTotals[intervalStart] += atRestTotal
			summary[satellite] += atRestTotal
			totalSummary += atRestTotal
		}
	}

	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		storageUsageDB := db.StorageUsage()

		t.Run("store", func(t *testing.T) {
			err := storageUsageDB.Store(ctx, stamps)
			assert.NoError(t, err)
		})

		t.Run("get daily", func(t *testing.T) {
			res, err := storageUsageDB.GetDaily(ctx, satelliteID, time.Time{}, now)
			assert.NoError(t, err)
			assert.NotNil(t, res)

			for _, stamp := range res {
				assert.Equal(t, satelliteID, stamp.SatelliteID)
				assert.Equal(t, expectedDailyStamps[satelliteID][stamp.IntervalStart].AtRestTotal, stamp.AtRestTotal)
			}
		})

		t.Run("get daily total", func(t *testing.T) {
			res, err := storageUsageDB.GetDailyTotal(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, days, len(res))

			for _, stamp := range res {
				// there can be inconsistencies in the values due to rounding off errors
				// and can make the test flaky.
				// rounding the values to 5 decimal places to avoid flakiness
				assert.Equal(t, roundFloat(expectedDailyStampsTotals[stamp.IntervalStart]), roundFloat(stamp.AtRestTotal))
			}
		})

		t.Run("summary satellite", func(t *testing.T) {
			summ, err := storageUsageDB.SatelliteSummary(ctx, satelliteID, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, roundFloat(summary[satelliteID]), roundFloat(summ))
		})

		t.Run("summary", func(t *testing.T) {
			summ, err := storageUsageDB.Summary(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, roundFloat(totalSummary), roundFloat(summ))
		})
	})
}

func TestEmptyStorageUsage(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		var emptySummary float64
		now := time.Now()

		storageUsageDB := db.StorageUsage()

		t.Run("get daily", func(t *testing.T) {
			res, err := storageUsageDB.GetDaily(ctx, storj.NodeID{}, time.Time{}, now)
			assert.NoError(t, err)
			assert.Nil(t, res)
		})

		t.Run("get daily total", func(t *testing.T) {
			res, err := storageUsageDB.GetDailyTotal(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.Nil(t, res)
		})

		t.Run("summary satellite", func(t *testing.T) {
			summ, err := storageUsageDB.SatelliteSummary(ctx, storj.NodeID{}, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, emptySummary, summ)
		})

		t.Run("summary", func(t *testing.T) {
			summ, err := storageUsageDB.Summary(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, emptySummary, summ)
		})
	})
}

func TestZeroStorageUsage(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		storageUsageDB := db.StorageUsage()
		now := time.Now().UTC()

		satelliteID := testrand.NodeID()
		stamp := storageusage.Stamp{
			SatelliteID:     satelliteID,
			AtRestTotal:     0,
			IntervalStart:   time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
			IntervalEndTime: now,
		}

		expectedStamp := []storageusage.Stamp{stamp}

		t.Run("store", func(t *testing.T) {
			err := storageUsageDB.Store(ctx, []storageusage.Stamp{stamp})
			assert.NoError(t, err)
		})

		t.Run("get daily", func(t *testing.T) {
			res, err := storageUsageDB.GetDaily(ctx, satelliteID, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, len(res), 1)
			assert.Equal(t, expectedStamp[0].AtRestTotal, res[0].AtRestTotal)
		})

		t.Run("get daily total", func(t *testing.T) {
			res, err := storageUsageDB.GetDailyTotal(ctx, time.Time{}, now)
			assert.NoError(t, err)
			assert.Equal(t, len(res), 1)
			assert.Equal(t, expectedStamp[0].AtRestTotal, res[0].AtRestTotal)
		})
	})
}

// RoundFloat rounds float value to 5 decimal places.
func roundFloat(value float64) float64 {
	return math.Round(value*100000) / 100000
}
