// Copyright (C) 2019 Storj Labs, Incache.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
)

// DownloadSelectionDB implements the database for download selection cache.
//
// architecture: Database
type DownloadSelectionDB interface {
	// SelectAllStorageNodesDownload returns nodes that are ready for downloading
	SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf AsOfSystemTimeConfig) ([]*SelectedNode, error)
}

// DownloadSelectionCacheConfig contains configuration for the selection cache.
type DownloadSelectionCacheConfig struct {
	Staleness      time.Duration
	OnlineWindow   time.Duration
	AsOfSystemTime AsOfSystemTimeConfig
}

// DownloadSelectionCache keeps a list of all the storage nodes that are qualified to download data from.
// The cache will sync with the nodes table in the database and get refreshed once the staleness time has past.
type DownloadSelectionCache struct {
	log    *zap.Logger
	db     DownloadSelectionDB
	config DownloadSelectionCacheConfig

	mu          sync.RWMutex
	lastRefresh time.Time
	state       *DownloadSelectionCacheState
}

// NewDownloadSelectionCache creates a new cache that keeps a list of all the storage nodes that are qualified to download data from.
func NewDownloadSelectionCache(log *zap.Logger, db DownloadSelectionDB, config DownloadSelectionCacheConfig) *DownloadSelectionCache {
	return &DownloadSelectionCache{
		log:    log,
		db:     db,
		config: config,
	}
}

// Refresh populates the cache with all of the reputableNodes and newNode nodes
// This method is useful for tests.
func (cache *DownloadSelectionCache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = cache.refresh(ctx)
	return err
}

// refresh calls out to the database and refreshes the cache with the most up-to-date
// data from the nodes table, then sets time that the last refresh occurred so we know when
// to refresh again in the future.
func (cache *DownloadSelectionCache) refresh(ctx context.Context) (state *DownloadSelectionCacheState, err error) {
	defer mon.Task()(&ctx)(&err)
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.state != nil && time.Since(cache.lastRefresh) <= cache.config.Staleness {
		return cache.state, nil
	}

	onlineNodes, err := cache.db.SelectAllStorageNodesDownload(ctx, cache.config.OnlineWindow, cache.config.AsOfSystemTime)
	if err != nil {
		return cache.state, err
	}

	cache.lastRefresh = time.Now().UTC()
	cache.state = NewDownloadSelectionCacheState(onlineNodes)

	mon.IntVal("refresh_cache_size_online").Observe(int64(len(onlineNodes)))
	return cache.state, nil
}

// GetNodeIPs gets the last node ip:port from the cache, refreshing when needed.
func (cache *DownloadSelectionCache) GetNodeIPs(ctx context.Context, nodes []storj.NodeID) (_ map[storj.NodeID]string, err error) {
	defer mon.Task()(&ctx)(&err)

	cache.mu.RLock()
	lastRefresh := cache.lastRefresh
	state := cache.state
	cache.mu.RUnlock()

	// if the cache is stale, then refresh it before we get nodes
	if state == nil || time.Since(lastRefresh) > cache.config.Staleness {
		state, err = cache.refresh(ctx)
		if err != nil {
			return nil, err
		}
	}

	return state.IPs(nodes), nil
}

// Size returns how many nodes are in the cache.
func (cache *DownloadSelectionCache) Size() int {
	cache.mu.RLock()
	state := cache.state
	cache.mu.RUnlock()

	if state == nil {
		return 0
	}

	return state.Size()
}

// DownloadSelectionCacheState contains state of download selection cache.
type DownloadSelectionCacheState struct {
	// ipPortByID returns IP based on storj.NodeID
	ipPortByID map[storj.NodeID]string
}

// NewDownloadSelectionCacheState creates a new state from the nodes.
func NewDownloadSelectionCacheState(nodes []*SelectedNode) *DownloadSelectionCacheState {
	ipPortByID := map[storj.NodeID]string{}
	for _, n := range nodes {
		ipPortByID[n.ID] = n.LastIPPort
	}
	return &DownloadSelectionCacheState{
		ipPortByID: ipPortByID,
	}
}

// Size returns how many nodes are in the state.
func (state *DownloadSelectionCacheState) Size() int {
	return len(state.ipPortByID)
}

// IPs returns node ip:port for nodes that are in state.
func (state *DownloadSelectionCacheState) IPs(nodes []storj.NodeID) map[storj.NodeID]string {
	xs := make(map[storj.NodeID]string, len(nodes))
	for _, nodeID := range nodes {
		if ip, exists := state.ipPortByID[nodeID]; exists {
			xs[nodeID] = ip
		}
	}
	return xs
}
