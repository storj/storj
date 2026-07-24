// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package storageusage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
)

func TestNormalizeForDisplayRecoveredUS1Gap(t *testing.T) {
	satelliteID := testrand.NodeID()
	stamps := []Stamp{
		{
			SatelliteID:      satelliteID,
			AtRestTotal:      9.74841403446273e12,
			AtRestTotalBytes: 1.2185517543078412e12,
			IntervalInHours:  8,
			IntervalStart:    date(2026, time.July, 8),
			IntervalEndTime:  time.Date(2026, time.July, 8, 6, 0, 0, 0, time.UTC),
		},
		{
			SatelliteID:     satelliteID,
			IntervalStart:   date(2026, time.July, 9),
			IntervalEndTime: time.Time{},
		},
		{
			SatelliteID:     satelliteID,
			IntervalStart:   date(2026, time.July, 10),
			IntervalEndTime: time.Time{},
		},
		{
			SatelliteID:      satelliteID,
			AtRestTotal:      77.8088065756294e12,
			AtRestTotalBytes: 4.3822659842311144e6,
			IntervalInHours:  17755382,
			IntervalStart:    date(2026, time.July, 11),
			IntervalEndTime:  time.Date(2026, time.July, 11, 14, 0, 0, 0, time.UTC),
		},
	}

	normalized := NormalizeForDisplay(
		stamps,
		date(2026, time.July, 8),
		time.Date(2026, time.July, 11, 23, 59, 0, 0, time.UTC),
	)

	require.Len(t, normalized, 4)
	expectedRecoveredRate := 77.8088065756294e12 / 80

	require.False(t, normalized[0].Calculated)
	require.InDelta(t, 1.2185517543078412e12, normalized[0].AtRestTotalBytes, 0.1)

	require.True(t, normalized[1].Calculated)
	require.True(t, normalized[2].Calculated)
	require.InDelta(t, expectedRecoveredRate, normalized[1].AtRestTotalBytes, 0.1)
	require.InDelta(t, expectedRecoveredRate, normalized[2].AtRestTotalBytes, 0.1)

	require.False(t, normalized[3].Calculated)
	require.Equal(t, float64(80), normalized[3].IntervalInHours)
	require.InDelta(t, expectedRecoveredRate, normalized[3].AtRestTotalBytes, 0.1)

	var rawBefore, rawAfter float64
	for _, stamp := range stamps {
		rawBefore += stamp.AtRestTotal
	}
	for _, stamp := range normalized {
		rawAfter += stamp.AtRestTotal
	}
	require.Equal(t, rawBefore, rawAfter)
}

func TestNormalizeForDisplayProjectsUnresolvedUS1Gap(t *testing.T) {
	satelliteID := testrand.NodeID()
	stamps := []Stamp{
		{
			SatelliteID:     satelliteID,
			AtRestTotal:     22.49257205835468e12,
			IntervalStart:   date(2026, time.July, 19),
			IntervalEndTime: time.Date(2026, time.July, 19, 13, 0, 0, 0, time.UTC),
		},
		{
			SatelliteID:     satelliteID,
			AtRestTotal:     12.066757977208667e12,
			IntervalStart:   date(2026, time.July, 20),
			IntervalEndTime: date(2026, time.July, 20),
		},
		{SatelliteID: satelliteID, IntervalStart: date(2026, time.July, 21)},
		{SatelliteID: satelliteID, IntervalStart: date(2026, time.July, 22)},
		{SatelliteID: satelliteID, IntervalStart: date(2026, time.July, 23)},
	}

	through := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	normalized := NormalizeForDisplay(stamps, date(2026, time.July, 19), through)

	require.Len(t, normalized, 6)
	expectedRate := 12.066757977208667e12 / 11
	for i := 2; i < len(normalized); i++ {
		require.True(t, normalized[i].Calculated)
		require.InDelta(t, expectedRate, normalized[i].AtRestTotalBytes, 0.1)
		require.Zero(t, normalized[i].AtRestTotal)
	}
	require.Equal(t, float64(12), normalized[len(normalized)-1].IntervalInHours)
}

func TestCombineForDisplayMarksCalculatedContributions(t *testing.T) {
	day := date(2026, time.July, 9)
	combined := CombineForDisplay(
		[]Stamp{{AtRestTotalBytes: 1e12, IntervalStart: day}},
		[]Stamp{{AtRestTotalBytes: 0.4e12, IntervalStart: day, Calculated: true}},
	)

	require.Len(t, combined, 1)
	require.Equal(t, 1.4e12, combined[0].AtRestTotalBytes)
	require.True(t, combined[0].Calculated)
	require.Equal(t, 1.4e12, DisplayAverage(combined))
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
