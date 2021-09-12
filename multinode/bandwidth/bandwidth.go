// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth

import (
	"sort"
	"time"
)

// Egress stores info about storage node egress usage.
type Egress struct {
	Repair int64 `json:"repair"`
	Audit  int64 `json:"audit"`
	Usage  int64 `json:"usage"`
}

// Ingress stores info about storage node ingress usage.
type Ingress struct {
	Repair int64 `json:"repair"`
	Usage  int64 `json:"usage"`
}

// UsageRollup contains rolluped bandwidth usage.
type UsageRollup struct {
	Egress        Egress    `json:"egress"`
	Ingress       Ingress   `json:"ingress"`
	Delete        int64     `json:"delete"`
	IntervalStart time.Time `json:"intervalStart"`
}

// Monthly contains all bandwidth, ingress, egress monthly data.
type Monthly struct {
	BandwidthDaily   []UsageRollup `json:"bandwidthDaily"`
	BandwidthSummary int64         `json:"bandwidthSummary"`
	EgressSummary    int64         `json:"egressSummary"`
	IngressSummary   int64         `json:"ingressSummary"`
}

// UsageRollupDailyCache caches storage usage stamps by interval date.
type UsageRollupDailyCache map[time.Time]UsageRollup

// Sorted returns usage rollup slice sorted by interval start.
func (cache *UsageRollupDailyCache) Sorted() []UsageRollup {
	var usageRollup []UsageRollup

	for _, stamp := range *cache {
		usageRollup = append(usageRollup, stamp)
	}
	sort.Slice(usageRollup, func(i, j int) bool {
		return usageRollup[i].IntervalStart.Before(usageRollup[j].IntervalStart)
	})

	return usageRollup
}

// Add adds usage rollup to cache aggregating bandwidth data by date.
func (cache *UsageRollupDailyCache) Add(rollup UsageRollup) {
	year, month, day := rollup.IntervalStart.UTC().Date()
	intervalStart := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	cached := *cache
	cacheStamp, ok := cached[intervalStart]
	if ok {
		cached[intervalStart] = UsageRollup{
			Egress: Egress{
				Repair: cacheStamp.Egress.Repair + rollup.Egress.Repair,
				Audit:  cacheStamp.Egress.Audit + rollup.Egress.Audit,
				Usage:  cacheStamp.Egress.Usage + rollup.Egress.Usage,
			},
			Ingress: Ingress{
				Repair: cacheStamp.Ingress.Repair + rollup.Ingress.Repair,
				Usage:  cacheStamp.Ingress.Usage + rollup.Ingress.Usage,
			},
			Delete:        cacheStamp.Delete + rollup.Delete,
			IntervalStart: intervalStart,
		}
	} else {
		cached[intervalStart] = UsageRollup{
			Egress: Egress{
				Repair: rollup.Egress.Repair,
				Audit:  rollup.Egress.Audit,
				Usage:  rollup.Egress.Usage,
			},
			Ingress: Ingress{
				Repair: rollup.Ingress.Repair,
				Usage:  rollup.Ingress.Usage,
			},
			Delete:        rollup.Delete,
			IntervalStart: intervalStart,
		}
	}
	*cache = cached
}
