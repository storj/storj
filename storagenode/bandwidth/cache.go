// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package bandwidth

import (
	"context"
	"sync"
	"time"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// Cache stores bandwidth usage in memory and persists it to the database.
// Currently, it only acts as a write cache.
type Cache struct {
	bandwidthdb DB
	hasNewData  bool

	usages map[CacheKey]*Usage

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

	created = created.UTC()
	created = time.Date(created.Year(), created.Month(), created.Day(), 0, 0, 0, 0, created.Location())

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

	return nil
}

// MonthSummary returns the summary of the current month's bandwidth usages.
func (c *Cache) MonthSummary(ctx context.Context, to time.Time) (int64, error) {
	err := c.Persist(ctx)
	if err != nil {
		return 0, err
	}

	return c.bandwidthdb.MonthSummary(ctx, to)
}

// Persist writes the cache to the database.
func (c *Cache) Persist(ctx context.Context) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hasNewData {
		err := c.bandwidthdb.AddBatch(ctx, c.usages)
		if err != nil {
			return err
		}
	}

	c.hasNewData = false
	c.usages = make(map[CacheKey]*Usage)

	return nil
}

// Summary returns the summary of bandwidth usages.
func (c *Cache) Summary(ctx context.Context, from, to time.Time) (*Usage, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.Summary(ctx, from, to)
}

// EgressSummary returns the summary of egress bandwidth usages.
func (c *Cache) EgressSummary(ctx context.Context, from, to time.Time) (*Usage, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.EgressSummary(ctx, from, to)
}

// IngressSummary returns the summary of ingress bandwidth usages.
func (c *Cache) IngressSummary(ctx context.Context, from, to time.Time) (*Usage, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.IngressSummary(ctx, from, to)
}

// SatelliteSummary returns the aggregated bandwidth usage for a particular satellite.
func (c *Cache) SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.SatelliteSummary(ctx, satelliteID, from, to)
}

// SatelliteEgressSummary returns the egress bandwidth usage for a particular satellite.
func (c *Cache) SatelliteEgressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.SatelliteEgressSummary(ctx, satelliteID, from, to)
}

// SatelliteIngressSummary returns the ingress bandwidth usage for a particular satellite.
func (c *Cache) SatelliteIngressSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (*Usage, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.SatelliteIngressSummary(ctx, satelliteID, from, to)
}

// SummaryBySatellite returns the summary of bandwidth usages by satellite.
func (c *Cache) SummaryBySatellite(ctx context.Context, from, to time.Time) (map[storj.NodeID]*Usage, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.SummaryBySatellite(ctx, from, to)
}

// GetDailyRollups returns the slice of daily bandwidth usage rollups for the provided time range, sorted in ascending order.
func (c *Cache) GetDailyRollups(ctx context.Context, from, to time.Time) ([]UsageRollup, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.GetDailyRollups(ctx, from, to)
}

// GetDailySatelliteRollups returns the slice of daily bandwidth usage for the provided time range, sorted in ascending order for a particular satellite.
func (c *Cache) GetDailySatelliteRollups(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) ([]UsageRollup, error) {
	err := c.Persist(ctx)
	if err != nil {
		return nil, err
	}

	return c.bandwidthdb.GetDailySatelliteRollups(ctx, satelliteID, from, to)
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
