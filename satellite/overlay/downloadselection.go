// Copyright (C) 2019 Storj Labs, Incache.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/nodeselection"
)

// DownloadSelectionDB implements the database for download selection cache.
//
// architecture: Database
type DownloadSelectionDB interface {
	// SelectAllStorageNodesDownload returns nodes that are ready for downloading
	SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf AsOfSystemTimeConfig) ([]*nodeselection.SelectedNode, error)
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

	cache          sync2.ReadCacheOf[*DownloadSelectionCacheState]
	placementRules nodeselection.PlacementRules
}

// NewDownloadSelectionCache creates a new cache that keeps a list of all the storage nodes that are qualified to download data from.
func NewDownloadSelectionCache(log *zap.Logger, db DownloadSelectionDB, placementRules nodeselection.PlacementRules, config DownloadSelectionCacheConfig) (*DownloadSelectionCache, error) {
	cache := &DownloadSelectionCache{
		log:            log,
		db:             db,
		placementRules: placementRules,
		config:         config,
	}
	return cache, cache.cache.Init(config.Staleness/2, config.Staleness, cache.read)
}

// Run runs the background task for cache.
func (cache *DownloadSelectionCache) Run(ctx context.Context) (err error) {
	return cache.cache.Run(ctx)
}

// Refresh populates the cache with all of the reputableNodes and newNode nodes
// This method is useful for tests.
func (cache *DownloadSelectionCache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = cache.cache.RefreshAndGet(ctx, time.Now())
	return err
}

// read loads the latest download selection state.
func (cache *DownloadSelectionCache) read(ctx context.Context) (_ *DownloadSelectionCacheState, err error) {
	defer mon.Task()(&ctx)(&err)

	onlineNodes, err := cache.db.SelectAllStorageNodesDownload(ctx, cache.config.OnlineWindow, cache.config.AsOfSystemTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	mon.IntVal("refresh_cache_size_online").Observe(int64(len(onlineNodes)))

	return NewDownloadSelectionCacheState(onlineNodes), nil
}

// GetNodeIPsFromPlacement gets the last node ip:port from the cache, refreshing when needed. Results are filtered out by placement.
func (cache *DownloadSelectionCache) GetNodeIPsFromPlacement(ctx context.Context, nodes []storj.NodeID, placement storj.PlacementConstraint) (_ map[storj.NodeID]string, err error) {
	defer mon.Task()(&ctx)(&err)

	state, err := cache.cache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	filter, _ := cache.placementRules(placement)

	return state.FilteredIPs(nodes, filter), nil
}

// GetNodes gets nodes by ID from the cache, and refreshes the cache if it is stale.
func (cache *DownloadSelectionCache) GetNodes(ctx context.Context, nodes []storj.NodeID) (_ map[storj.NodeID]*nodeselection.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	state, err := cache.cache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return state.Nodes(nodes), nil
}

// GetNode gets a node by ID from the cache, and refreshes the cache if it is stale.
func (cache *DownloadSelectionCache) GetNode(ctx context.Context, nodeID storj.NodeID) (_ *nodeselection.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	state, err := cache.cache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	selected, ok := state.byID[nodeID]
	if !ok {
		return nil, Error.New("node not found")
	}
	return selected.Clone(), nil
}

// Size returns how many nodes are in the cache.
func (cache *DownloadSelectionCache) Size(ctx context.Context) (int, error) {
	state, err := cache.cache.Get(ctx, time.Now())
	if state == nil || err != nil {
		return 0, Error.Wrap(err)
	}
	return state.Size(), nil
}

// DownloadSelectionCacheState contains state of download selection cache.
type DownloadSelectionCacheState struct {
	// byID returns IP based on storj.NodeID
	byID map[storj.NodeID]*nodeselection.SelectedNode // TODO: optimize, avoid pointery structures for performance
}

// NewDownloadSelectionCacheState creates a new state from the nodes.
func NewDownloadSelectionCacheState(nodes []*nodeselection.SelectedNode) *DownloadSelectionCacheState {
	byID := map[storj.NodeID]*nodeselection.SelectedNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	return &DownloadSelectionCacheState{
		byID: byID,
	}
}

// Size returns how many nodes are in the state.
func (state *DownloadSelectionCacheState) Size() int {
	return len(state.byID)
}

// IPs returns node ip:port for nodes that are in state.
func (state *DownloadSelectionCacheState) IPs(nodes []storj.NodeID) map[storj.NodeID]string {
	xs := make(map[storj.NodeID]string, len(nodes))
	for _, nodeID := range nodes {
		if n, exists := state.byID[nodeID]; exists {
			xs[nodeID] = n.LastIPPort
		}
	}
	return xs
}

// FilteredIPs returns node ip:port for nodes that are in state. Results are filtered out..
func (state *DownloadSelectionCacheState) FilteredIPs(nodes []storj.NodeID, filter nodeselection.NodeFilter) map[storj.NodeID]string {
	xs := make(map[storj.NodeID]string, len(nodes))
	for _, nodeID := range nodes {
		if n, exists := state.byID[nodeID]; exists && filter.Match(n) {
			xs[nodeID] = n.LastIPPort
		}
	}
	return xs
}

// Nodes returns node ip:port for nodes that are in state.
func (state *DownloadSelectionCacheState) Nodes(nodes []storj.NodeID) map[storj.NodeID]*nodeselection.SelectedNode {
	xs := make(map[storj.NodeID]*nodeselection.SelectedNode, len(nodes))
	for _, nodeID := range nodes {
		if n, exists := state.byID[nodeID]; exists {
			xs[nodeID] = n.Clone() // TODO: optimize the clones
		}
	}
	return xs
}
