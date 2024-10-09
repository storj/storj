// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth

import (
	"context"
	"sort"
	"sync"
	"time"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
)

// Cache stores bandwidth usage in memory and persists it to the database.
// Currently, it only acts as a write cache.
type Cache struct {
	bandwidthdb DB
	hasNewData  bool

	usages    map[CacheKey]*Usage
	startDate *time.Time

	mu sync.Mutex
}

// CacheKey is a key for the bandwidth cache.
type CacheKey struct {
	SatelliteID storj.NodeID
	CreatedAt   time.Time
}

// NewCache creates a new bandwidth Cache.
func NewCache(bandwidthdb DB) *Cache {
	return &Cache{
		usages:      make(map[CacheKey]*Usage),
		bandwidthdb: bandwidthdb,
	}
}

// Add adds a bandwidth usage to the cache.
func (c *Cache) Add(ctx context.Context, satelliteID storj.NodeID, action pb.PieceAction, amount int64, created time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	c.mu.Lock()
	defer c.mu.Unlock()

	created = toBeginningOfDay(created.UTC())

	key := CacheKey{
		SatelliteID: satelliteID,
		CreatedAt:   created,
	}

	usage := c.usages[key]

	if usage == nil {
		usage = &Usage{}
		c.usages[key] = usage
	}

	usage.Include(action, amount)
	c.hasNewData = true

	if c.startDate == nil || created.Before(*c.startDate) {
		c.startDate = &created
	}

	return nil
}

// MonthSummary returns the summary of the current month's bandwidth usages.
func (c *Cache) MonthSummary(ctx context.Context, now time.Time) (int64, error) {
	from, to := date.MonthBoundary(now.UTC())

	var summary int64

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if inTimeSpan(from, to, key.CreatedAt) {
				summary += u.Total()
			}
		}
	}
	c.mu.Unlock()

	summaryFromDB, err := c.bandwidthdb.MonthSummary(ctx, now)
	if err != nil {
		return 0, err
	}

	return summary + summaryFromDB, nil
}

// Persist writes the cache to the database.
func (c *Cache) Persist(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var usages map[CacheKey]*Usage
	c.mu.Lock()
	if c.hasNewData {
		usages = c.usages
		c.usages = make(map[CacheKey]*Usage)
		c.hasNewData = false
		c.startDate = nil
	}
	c.mu.Unlock()

	if len(usages) > 0 {
		err := c.bandwidthdb.AddBatch(ctx, usages)
		if err != nil {
			return err
		}
	}

	return nil
}

// Summary returns the summary of bandwidth usages.
func (c *Cache) Summary(ctx context.Context, from, to time.Time) (*Usage, error) {
	usage := Usage{}

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if inTimeSpan(from, to, key.CreatedAt) {
				usage.Add(u)
			}
		}
	}
	c.mu.Unlock()

	u, err := c.bandwidthdb.Summary(ctx, from, to)
	if err != nil {
		return nil, err
	}

	usage.Add(u)
	return &usage, nil
}

// EgressSummary returns the summary of egress bandwidth usages.
func (c *Cache) EgressSummary(ctx context.Context, from, to time.Time) (*Usage, error) {
	usage := Usage{}

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if inTimeSpan(from, to, key.CreatedAt) {
				usage.Add(u.Egress())
			}
		}
	}
	c.mu.Unlock()

	egress, err := c.bandwidthdb.EgressSummary(ctx, from, to)
	if err != nil {
		return nil, err
	}

	usage.Add(egress)
	return &usage, nil
}

// IngressSummary returns the summary of ingress bandwidth usages.
func (c *Cache) IngressSummary(ctx context.Context, from, to time.Time) (*Usage, error) {
	usage := Usage{}

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if inTimeSpan(from, to, key.CreatedAt) {
				usage.Add(u.Ingress())
			}
		}
	}
	c.mu.Unlock()

	ingress, err := c.bandwidthdb.IngressSummary(ctx, from, to)
	if err != nil {
		return nil, err
	}

	usage.Add(ingress)

	return &usage, nil
}

// SatelliteSummary returns the aggregated bandwidth usage for a particular satellite.
func (c *Cache) SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error) {
	var usage Usage

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if key.SatelliteID == satelliteID && inTimeSpan(from, to, key.CreatedAt) {
				usage.Add(u)
			}
		}
	}
	c.mu.Unlock()

	summary, err := c.bandwidthdb.SatelliteSummary(ctx, satelliteID, from, to)
	if err != nil {
		return nil, err
	}

	usage.Add(summary)

	return &usage, nil
}

// SatelliteEgressSummary returns the egress bandwidth usage for a particular satellite.
func (c *Cache) SatelliteEgressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error) {
	var usage Usage

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if key.SatelliteID == satelliteID && inTimeSpan(from, to, key.CreatedAt) {
				usage.Add(u.Egress())
			}
		}
	}
	c.mu.Unlock()

	summary, err := c.bandwidthdb.SatelliteEgressSummary(ctx, satelliteID, from, to)
	if err != nil {
		return nil, err
	}

	usage.Add(summary)

	return &usage, nil
}

// SatelliteIngressSummary returns the ingress bandwidth usage for a particular satellite.
func (c *Cache) SatelliteIngressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error) {
	var usage Usage

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if key.SatelliteID == satelliteID && inTimeSpan(from, to, key.CreatedAt) {
				usage.Add(u.Ingress())
			}
		}
	}
	c.mu.Unlock()

	summary, err := c.bandwidthdb.SatelliteIngressSummary(ctx, satelliteID, from, to)
	if err != nil {
		return nil, err
	}

	usage.Add(summary)

	return &usage, nil
}

// SummaryBySatellite returns the summary of bandwidth usages by satellite.
func (c *Cache) SummaryBySatellite(ctx context.Context, from, to time.Time) (map[storj.NodeID]*Usage, error) {
	usage := make(map[storj.NodeID]*Usage)

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if inTimeSpan(from, to, key.CreatedAt) {
				if _, ok := usage[key.SatelliteID]; !ok {
					usage[key.SatelliteID] = &Usage{}
				}
				usage[key.SatelliteID].Add(u)
			}
		}
	}
	c.mu.Unlock()

	summary, err := c.bandwidthdb.SummaryBySatellite(ctx, from, to)
	if err != nil {
		return nil, err
	}

	for satelliteID, u := range summary {
		if _, ok := usage[satelliteID]; !ok {
			usage[satelliteID] = &Usage{}
		}
		usage[satelliteID].Add(u)
	}

	return usage, nil
}

// GetDailyRollups returns the slice of daily bandwidth usage rollups for the provided time range, sorted in ascending order.
func (c *Cache) GetDailyRollups(ctx context.Context, from, to time.Time) ([]UsageRollup, error) {
	usageByDay := make(map[time.Time]*UsageRollup)

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if inTimeSpan(from, to, key.CreatedAt) {
				day := key.CreatedAt
				if _, ok := usageByDay[day]; !ok {
					usageByDay[day] = &UsageRollup{IntervalStart: day}
				}

				usageByDay[day].Egress.Add(u.ToEgress())
				usageByDay[day].Ingress.Add(u.ToIngress())
				usageByDay[day].Delete += u.Delete
			}
		}
	}
	c.mu.Unlock()

	rollups, err := c.bandwidthdb.GetDailyRollups(ctx, from, to)
	if err != nil {
		return nil, err
	}

	for _, rollup := range rollups {
		if _, ok := usageByDay[rollup.IntervalStart]; !ok {
			usageByDay[rollup.IntervalStart] = &UsageRollup{IntervalStart: rollup.IntervalStart}
		}
		usageByDay[rollup.IntervalStart].Egress.Add(rollup.Egress)
		usageByDay[rollup.IntervalStart].Ingress.Add(rollup.Ingress)
		usageByDay[rollup.IntervalStart].Delete += rollup.Delete
	}

	var result []UsageRollup
	for _, rollup := range usageByDay {
		result = append(result, *rollup)
	}

	// sort the result by interval start in ascending order
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].IntervalStart.Before(result[j].IntervalStart)
	})

	return result, nil
}

// GetDailySatelliteRollups returns the slice of daily bandwidth usage for the provided time range, sorted in ascending order for a particular satellite.
func (c *Cache) GetDailySatelliteRollups(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) ([]UsageRollup, error) {
	usageByDay := make(map[time.Time]*UsageRollup)

	from = from.UTC()
	to = to.UTC()

	c.mu.Lock()
	if c.startDate != nil && inTimeSpan(from, to, *c.startDate) {
		for key, u := range c.usages {
			if key.SatelliteID == satelliteID && inTimeSpan(from, to, key.CreatedAt) {
				day := key.CreatedAt
				if _, ok := usageByDay[day]; !ok {
					usageByDay[day] = &UsageRollup{IntervalStart: day}
				}

				usageByDay[day].Egress.Add(u.ToEgress())
				usageByDay[day].Ingress.Add(u.ToIngress())
				usageByDay[day].Delete += u.Delete
			}
		}
	}
	c.mu.Unlock()

	rollups, err := c.bandwidthdb.GetDailySatelliteRollups(ctx, satelliteID, from, to)
	if err != nil {
		return nil, err
	}

	for _, rollup := range rollups {
		if _, ok := usageByDay[rollup.IntervalStart]; !ok {
			usageByDay[rollup.IntervalStart] = &UsageRollup{IntervalStart: rollup.IntervalStart}
		}
		usageByDay[rollup.IntervalStart].Egress.Add(rollup.Egress)
		usageByDay[rollup.IntervalStart].Ingress.Add(rollup.Ingress)
		usageByDay[rollup.IntervalStart].Delete += rollup.Delete
	}

	var result []UsageRollup
	for _, rollup := range usageByDay {
		result = append(result, *rollup)
	}

	// sort the result by interval start in ascending order
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].IntervalStart.Before(result[j].IntervalStart)
	})

	return result, nil
}

// AddBatch adds a batch of bandwidth usages to the cache.
func (c *Cache) AddBatch(ctx context.Context, usages map[CacheKey]*Usage) (err error) {
	defer mon.Task()(&ctx)(&err)

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, usage := range usages {
		tUsage, ok := c.usages[key]
		if !ok {
			tUsage = &Usage{}
			c.usages[key] = tUsage
		}
		tUsage.Add(usage)
	}

	return nil
}

// inTimeSpan checks if `from <= t <= to`.
func inTimeSpan(from, to, t time.Time) bool {
	return from.Compare(t) <= 0 && t.Compare(to) <= 0
}

// toBeginningOfDay returns the beginning of the day for the provided time.
func toBeginningOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
