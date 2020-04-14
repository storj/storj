// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
)

var allObserverStatsCollectors = newObserverStatsCollectors()

type observerStatsCollectors struct {
	mu       sync.Mutex
	observer map[string]*observerStats
}

func newObserverStatsCollectors() *observerStatsCollectors {
	return &observerStatsCollectors{
		observer: make(map[string]*observerStats),
	}
}

func (list *observerStatsCollectors) GetStats(name string) *observerStats {
	list.mu.Lock()
	defer list.mu.Unlock()

	stats, ok := list.observer[name]
	if !ok {
		stats = newObserverStats(name)
		mon.Chain(stats)
		list.observer[name] = stats
	}
	return stats
}

// observerStats tracks the most recent observer stats.
type observerStats struct {
	mu sync.Mutex

	key    monkit.SeriesKey
	total  time.Duration
	object *monkit.DurationDist
	inline *monkit.DurationDist
	remote *monkit.DurationDist
}

func newObserverStats(name string) *observerStats {
	return &observerStats{
		key:    monkit.NewSeriesKey("observer").WithTag("name", name),
		total:  0,
		object: nil,
		inline: nil,
		remote: nil,
	}
}

func (stats *observerStats) Observe(observer *observerContext) {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.total = observer.object.Sum + observer.inline.Sum + observer.remote.Sum
	stats.object = observer.object
	stats.inline = observer.inline
	stats.remote = observer.remote
}

func (stats *observerStats) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	cb(stats.key, "sum", stats.total.Seconds())

	if stats.object != nil {
		stats.object.Stats(cb)
	}
	if stats.inline != nil {
		stats.inline.Stats(cb)
	}
	if stats.remote != nil {
		stats.remote.Stats(cb)
	}
}
