// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"sort"
	"time"
)

// UsageStamp holds data at rest total for an interval beginning at interval start.
type UsageStamp struct {
	AtRestTotal   float64
	IntervalStart time.Time
}

// UsageStampDailyCache caches storage usage stamps by interval date.
type UsageStampDailyCache map[time.Time]UsageStamp

// Add adds usage stamp to cache aggregating at rest data by date.
func (cache *UsageStampDailyCache) Add(stamp UsageStamp) {
	year, month, day := stamp.IntervalStart.UTC().Date()
	intervalStart := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	cached := *cache

	cacheStamp, ok := cached[intervalStart]
	if ok {
		cached[intervalStart] = UsageStamp{
			AtRestTotal:   cacheStamp.AtRestTotal + stamp.AtRestTotal,
			IntervalStart: intervalStart,
		}
	} else {
		cached[intervalStart] = UsageStamp{
			AtRestTotal:   stamp.AtRestTotal,
			IntervalStart: intervalStart,
		}
	}

	*cache = cached
}

// Sorted returns usage stamp slice sorted by interval start.
func (cache *UsageStampDailyCache) Sorted() []UsageStamp {
	var usage []UsageStamp

	for _, stamp := range *cache {
		usage = append(usage, stamp)
	}
	sort.Slice(usage, func(i, j int) bool {
		return usage[i].IntervalStart.Before(usage[j].IntervalStart)
	})

	return usage
}
