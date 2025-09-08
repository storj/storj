// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// SuccessTrackerMonitor is a monkit source, which publishes success scores.
type SuccessTrackerMonitor struct {
	log       *zap.Logger
	overlayDB overlay.DB
	filter    nodeselection.NodeFilter
	cache     *sync2.ReadCacheOf[map[storj.NodeID]*nodeselection.SelectedNode]
	mu        sync.Mutex
	trackers  map[monkit.SeriesKey]SuccessTracker
	enabled   bool
}

// NewSuccessTrackerMonitor creates a new monitor for tracking node success/failure metrics.
func NewSuccessTrackerMonitor(log *zap.Logger, db overlay.DB, cfg Config) (tracker *SuccessTrackerMonitor, err error) {
	filter, err := nodeselection.FilterFromString(cfg.SuccessTrackerMonitorFilter, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	tracker = &SuccessTrackerMonitor{
		log:       log,
		overlayDB: db,
		filter:    filter,
		enabled:   cfg.SuccessTrackerMonitorEnabled,
	}
	tracker.cache, err = sync2.NewReadCache(15*time.Minute, time.Hour, tracker.refreshNodes)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if cfg.SuccessTrackerMonitorEnabled {
		mon.Chain(tracker)
	}
	return tracker, nil
}

// Run starts the background task for the success tracker monitor, which periodically refreshes node data.
func (s *SuccessTrackerMonitor) Run(ctx context.Context) error {
	if !s.enabled {
		return nil
	}
	return s.cache.Run(ctx)
}

// Stats iterates through all registered trackers and reports their metrics via the callback.
func (s *SuccessTrackerMonitor) Stats(cb func(key monkit.SeriesKey, field string, val float64)) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	nodes, err := s.cache.Get(ctx, time.Now())
	if err != nil {
		s.log.Warn("failed to fetch nodes for success/failure score reporting", zap.Error(err))
		return
	}
	for key, tracker := range s.trackers {
		tracker.Range(func(id storj.NodeID, f float64) {
			node, found := nodes[id]
			if !found {
				return
			}
			if !s.filter.Match(node) {
				return
			}
			cb(key.WithTag("node_id", hex.EncodeToString(id.Bytes())), "recent", f)
		})
	}
}

// RegisterTracker registers a success tracker with the monitor for metric collection.
func (s *SuccessTrackerMonitor) RegisterTracker(key monkit.SeriesKey, tracker SuccessTracker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trackers == nil {
		s.trackers = make(map[monkit.SeriesKey]SuccessTracker)
	}
	s.trackers[key] = tracker
}

func (s *SuccessTrackerMonitor) refreshNodes(ctx context.Context) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
	nodes, err := s.overlayDB.GetAllParticipatingNodes(ctx, 24*time.Hour, -10*time.Second)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	result := make(map[storj.NodeID]*nodeselection.SelectedNode, len(nodes))
	for _, node := range nodes {
		result[node.ID] = &node
	}
	return result, nil
}
