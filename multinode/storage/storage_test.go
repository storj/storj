// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/multinode/storage"
)

func TestUsageStampDailyCache(t *testing.T) {
	newTimestamp := func(month time.Month, day, hour int) time.Time {
		return time.Date(2021, month, day, hour, 0, 0, 0, time.UTC)
	}

	testData := []struct {
		Date   time.Time
		AtRest []float64
		Hours  []int
	}{
		{
			Date:   newTimestamp(time.May, 1, 0),
			AtRest: []float64{2, 3},
			Hours:  []int{1, 0},
		},
		{
			Date:   newTimestamp(time.May, 2, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 3, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 4, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 5, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 6, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 7, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 8, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 9, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 10, 0),
			AtRest: []float64{1, 2, 3},
			Hours:  []int{0, 1, 0},
		},
		{
			Date:   newTimestamp(time.May, 11, 0),
			AtRest: []float64{1, 2},
			Hours:  []int{0, 1},
		},
		{
			Date:   newTimestamp(time.May, 12, 0),
			AtRest: []float64{1, 2},
			Hours:  []int{0, 1},
		},
	}

	expected := []storage.UsageStamp{
		{
			IntervalStart: newTimestamp(time.May, 1, 0),
			AtRestTotal:   5,
		},
		{
			IntervalStart: newTimestamp(time.May, 2, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 3, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 4, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 5, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 6, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 7, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 8, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 9, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 10, 0),
			AtRestTotal:   6,
		},
		{
			IntervalStart: newTimestamp(time.May, 11, 0),
			AtRestTotal:   3,
		},
		{
			IntervalStart: newTimestamp(time.May, 12, 0),
			AtRestTotal:   3,
		},
	}

	cache := make(storage.UsageStampDailyCache)
	for _, entry := range testData {
		_, month, day := entry.Date.Date()

		for i, atRest := range entry.AtRest {
			cache.Add(storage.UsageStamp{
				AtRestTotal:   atRest,
				IntervalStart: newTimestamp(month, day, entry.Hours[i]),
			})
		}
	}

	stamps := cache.Sorted()
	require.Equal(t, expected, stamps)
}

func TestDiskSpaceAdd(t *testing.T) {
	totalDiskSpace := storage.DiskSpace{
		Allocated: 0,
		Used:      0,
		UsedTrash: 0,
		Free:      0,
		Available: 0,
		Overused:  0,
	}

	testData := []struct {
		diskSpace storage.DiskSpace
		expected  storage.DiskSpace
	}{
		{
			diskSpace: storage.DiskSpace{
				Allocated: 1,
				Used:      1,
				UsedTrash: 1,
				Free:      1,
				Available: 1,
				Overused:  1,
			},
			expected: storage.DiskSpace{
				Allocated: 1,
				Used:      1,
				UsedTrash: 1,
				Free:      1,
				Available: 1,
				Overused:  1,
			},
		},
		{
			diskSpace: storage.DiskSpace{
				Allocated: 2,
				Used:      2,
				UsedTrash: 2,
				Free:      2,
				Available: 2,
				Overused:  2,
			},
			expected: storage.DiskSpace{
				Allocated: 3,
				Used:      3,
				UsedTrash: 3,
				Free:      3,
				Available: 3,
				Overused:  3,
			},
		},
		{
			diskSpace: storage.DiskSpace{
				Allocated: 3,
				Used:      3,
				UsedTrash: 3,
				Free:      3,
				Available: 3,
				Overused:  3,
			},
			expected: storage.DiskSpace{
				Allocated: 6,
				Used:      6,
				UsedTrash: 6,
				Free:      6,
				Available: 6,
				Overused:  6,
			},
		},
	}

	for _, test := range testData {
		totalDiskSpace.Add(test.diskSpace)
		assert.Equal(t, totalDiskSpace, test.expected)
	}
}
