// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/multinode/bandwidth"
)

func TestUsageRolloutDailyCache(t *testing.T) {
	newTimestamp := func(month time.Month, day int) time.Time {
		return time.Date(2021, month, day, 0, 0, 0, 0, time.UTC)
	}

	testData := []struct {
		Date    time.Time
		Egress  []bandwidth.Egress
		Ingress []bandwidth.Ingress
		Delete  []int64
	}{
		{
			Date:    newTimestamp(time.May, 2),
			Ingress: []bandwidth.Ingress{{Repair: 1, Usage: 0}, {Repair: 10, Usage: 20}},
			Egress:  []bandwidth.Egress{{Repair: 10, Audit: 20, Usage: 30}, {Repair: 10, Audit: 20, Usage: 30}},
			Delete:  []int64{10, 20, 30},
		},
		{
			Date:    newTimestamp(time.May, 3),
			Ingress: []bandwidth.Ingress{{Repair: 1, Usage: 0}, {Repair: 10, Usage: 20}},
			Egress:  []bandwidth.Egress{{Repair: 101, Audit: 201, Usage: 301}, {Repair: 101, Audit: 201, Usage: 301}},
			Delete:  []int64{101, 201, 301},
		},
		{
			Date:    newTimestamp(time.May, 4),
			Ingress: []bandwidth.Ingress{{Repair: 12, Usage: 20}, {Repair: 120, Usage: 220}},
			Egress:  []bandwidth.Egress{{Repair: 310, Audit: 320, Usage: 330}, {Repair: 100, Audit: 200, Usage: 300}},
			Delete:  []int64{100, 200, 300},
		},
		{
			Date:    newTimestamp(time.May, 1),
			Ingress: []bandwidth.Ingress{{Repair: 123, Usage: 123}, {Repair: 123, Usage: 123}},
			Egress:  []bandwidth.Egress{{Repair: 20, Audit: 20, Usage: 20}, {Repair: 30, Audit: 30, Usage: 30}},
			Delete:  []int64{2, 3, 4},
		},
	}
	expected := []bandwidth.UsageRollup{
		{
			IntervalStart: newTimestamp(time.May, 1),
			Ingress:       bandwidth.Ingress{Repair: 246, Usage: 246},
			Egress:        bandwidth.Egress{Repair: 50, Audit: 50, Usage: 50},
			Delete:        9,
		},
		{
			IntervalStart: newTimestamp(time.May, 2),
			Ingress:       bandwidth.Ingress{Repair: 11, Usage: 20},
			Egress:        bandwidth.Egress{Repair: 20, Audit: 40, Usage: 60},
			Delete:        60,
		},
		{
			IntervalStart: newTimestamp(time.May, 3),
			Ingress:       bandwidth.Ingress{Repair: 11, Usage: 20},
			Egress:        bandwidth.Egress{Repair: 202, Audit: 402, Usage: 602},
			Delete:        603,
		},
		{
			IntervalStart: newTimestamp(time.May, 4),
			Ingress:       bandwidth.Ingress{Repair: 132, Usage: 240},
			Egress:        bandwidth.Egress{Repair: 410, Audit: 520, Usage: 630},
			Delete:        600,
		},
	}

	cache := make(bandwidth.UsageRollupDailyCache)
	for _, entry := range testData {
		_, month, day := entry.Date.Date()
		for _, egr := range entry.Egress {
			cache.Add(bandwidth.UsageRollup{
				Egress:        egr,
				IntervalStart: newTimestamp(month, day),
			})
		}
		for _, ing := range entry.Ingress {
			cache.Add(bandwidth.UsageRollup{
				Ingress:       ing,
				IntervalStart: newTimestamp(month, day),
			})
		}
		for _, del := range entry.Delete {
			cache.Add(bandwidth.UsageRollup{
				Delete:        del,
				IntervalStart: newTimestamp(month, day),
			})
		}
	}

	stamps := cache.Sorted()
	require.Equal(t, expected, stamps)
}
