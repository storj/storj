package overlay

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
)

type downtimeInfo struct {
	start time.Time
	end   time.Time
}

type PlannedDowntimeNodesDB interface {
	SelectNodeWithPlannedDowntime(ctx context.Context, endAfter time.Time) ([]NodePlannedDowntime, error)
}

type PlannedDowntimeCache struct {
	log       *zap.Logger
	db        PlannedDowntimeNodesDB
	staleness time.Duration

	mu          sync.RWMutex
	lastRefresh time.Time
	state       map[storj.NodeID]downtimeInfo
}

func NewPlannedDowntimeCache(log *zap.Logger, db PlannedDowntimeNodesDB, staleness time.Duration) *PlannedDowntimeCache {
	return &PlannedDowntimeCache{
		log: log,
		db:  db,
	}
}

// Refresh populates the cache with all of nodes that has future downtime planned.
// This method is useful for tests.
func (cache *PlannedDowntimeCache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = cache.refresh(ctx)
	return err
}

func (cache *PlannedDowntimeCache) GetScheduled(ctx context.Context, endAfter time.Time) (_ map[storj.NodeID]NodePlannedDowntime, err error) {
	defer mon.Task()(&ctx)(nil)

	cache.mu.RLock()
	lastRefresh := cache.lastRefresh
	state := cache.state
	cache.mu.RUnlock()

	// if the cache is stale, then refresh it before we get nodes
	if state == nil || time.Since(lastRefresh) > cache.staleness {
		state, err = cache.refresh(ctx)
		if err != nil {
			return nil, err
		}
	}

	results := make(map[storj.NodeID]NodePlannedDowntime)
	for id, info := range cache.state {
		if info.end.After(endAfter.UTC()) && info.start.Before(endAfter.UTC()) {
			results[id] = NodePlannedDowntime{
				ID:    id,
				Start: info.start,
				End:   info.end,
			}
		}
	}

	cache.log.Debug(`========================
	PlannedDowntime
	======================`, zap.Any("GetScheduled", len(cache.state)))

	return results, nil

}

func (cache *PlannedDowntimeCache) refresh(ctx context.Context) (map[storj.NodeID]downtimeInfo, error) {
	defer mon.Task()(&ctx)(nil)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.state != nil && time.Since(cache.lastRefresh) <= cache.staleness {
		return cache.state, nil
	}

	nodes, err := cache.db.SelectNodeWithPlannedDowntime(ctx, time.Now())
	if err != nil {
		return cache.state, err
	}

	cache.lastRefresh = time.Now().UTC()
	cache.state = make(map[storj.NodeID]downtimeInfo)
	for _, n := range nodes {
		cache.state[n.ID] = downtimeInfo{
			start: n.Start,
			end:   n.End,
		}
	}

	return cache.state, nil
}
