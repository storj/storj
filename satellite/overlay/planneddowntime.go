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
	ScheduleDowntime(id storj.NodeID, start, end time.Time) (*NodePlannedDowntime, error)
	RemoveDowntime(id storj.NodeID) error
}

type PlannedDowntimeCache struct {
	log       *zap.Logger
	db        PlannedDowntimeNodesDB
	staleness time.Duration

	mu          sync.RWMutex
	lastRefresh time.Time
	// TODO: it's probably better that this is sorted from most recent to
	// upcoming later
	state map[storj.NodeID][]downtimeInfo
}

func NewPlannedDowntimeCache(log *zap.Logger, db PlannedDowntimeNodesDB, staleness time.Duration) *PlannedDowntimeCache {
	return &PlannedDowntimeCache{
		log: log,
		db:  db,
	}
}

func (cache *PlannedDowntimeCache) Add(ctx context.Context, info NodePlannedDowntime) (err error) {
	defer mon.Task()(&ctx)(&err)

	cache.refresh(ctx)

	_, err = cache.db.ScheduleDowntime(info.ID, info.Start, info.End)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	var infos []downtimeInfo
	var ok bool
	if infos, ok = cache.state[info.ID]; !ok {
		infos = make([]downtimeInfo, 0)
	}
	cache.state[info.ID] = append(infos, downtimeInfo{start: info.Start, end: info.End})

	return err
}

func (cache *PlannedDowntimeCache) Delete(ctx context.Context, pd NodePlannedDowntime) (err error) {
	defer mon.Task()(&ctx)(&err)

	cache.refresh(ctx)

	err = cache.db.RemoveDowntime(pd.ID)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	for n, infos := range cache.state {
		if n == pd.ID {
			for i, info := range infos {
				if info.start == pd.Start && info.end == pd.End {
					cache.state[n] = append(infos[:i], infos[i+1:]...)
				}
			}
		}
	}

	return nil
}

func (cache *PlannedDowntimeCache) GetScheduled(ctx context.Context, endAfter time.Time) map[storj.NodeID]NodePlannedDowntime {
	defer mon.Task()(&ctx)(nil)

	cache.mu.RLocker()
	defer cache.mu.RLocker().Unlock()

	var results map[storj.NodeID]NodePlannedDowntime
	for id, infos := range cache.state {
		for _, info := range infos {
			if info.end.After(endAfter.UTC()) && info.start.Before(endAfter.UTC()) {
				results[id] = NodePlannedDowntime{
					ID:    id,
					Start: info.start,
					End:   info.end,
				}
			}
		}
	}

	return results

}

func (cache *PlannedDowntimeCache) refresh(ctx context.Context) {
	defer mon.Task()(&ctx)(nil)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if time.Since(cache.lastRefresh) <= cache.staleness {
		return
	}

	now := time.Now().UTC()
	// delete entries that its end time has passed
	for n, infos := range cache.state {
		for i, info := range infos {
			if time.Since(info.end) > 0 {
				cache.state[n] = append(infos[:i], infos[i+1:]...)
			}
		}
	}

	cache.lastRefresh = now

	return
}
