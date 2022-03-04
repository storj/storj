// Copyright (C) 2019 Storj Labs, Incache.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection/uploadselection"
)

// UploadSelectionDB implements the database for upload selection cache.
//
// architecture: Database
type UploadSelectionDB interface {
	// SelectAllStorageNodesUpload returns all nodes that qualify to store data, organized as reputable nodes and new nodes
	SelectAllStorageNodesUpload(ctx context.Context, selectionCfg NodeSelectionConfig) (reputable, new []*SelectedNode, err error)
}

// UploadSelectionCacheConfig is a configuration for upload selection cache.
type UploadSelectionCacheConfig struct {
	Disabled  bool          `help:"disable node cache" default:"false"`
	Staleness time.Duration `help:"how stale the node selection cache can be" releaseDefault:"3m" devDefault:"5m" testDefault:"3m"`
}

// UploadSelectionCache keeps a list of all the storage nodes that are qualified to store data
// We organize the nodes by if they are reputable or a new node on the network.
// The cache will sync with the nodes table in the database and get refreshed once the staleness time has past.
type UploadSelectionCache struct {
	log             *zap.Logger
	db              UploadSelectionDB
	selectionConfig NodeSelectionConfig
	staleness       time.Duration

	mu          sync.RWMutex
	lastRefresh time.Time
	state       *uploadselection.State
}

// NewUploadSelectionCache creates a new cache that keeps a list of all the storage nodes that are qualified to store data.
func NewUploadSelectionCache(log *zap.Logger, db UploadSelectionDB, staleness time.Duration, config NodeSelectionConfig) *UploadSelectionCache {
	return &UploadSelectionCache{
		log:             log,
		db:              db,
		staleness:       staleness,
		selectionConfig: config,
	}
}

// Refresh populates the cache with all of the reputableNodes and newNode nodes
// This method is useful for tests.
func (cache *UploadSelectionCache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = cache.refresh(ctx)
	return err
}

// refresh calls out to the database and refreshes the cache with the most up-to-date
// data from the nodes table, then sets time that the last refresh occurred so we know when
// to refresh again in the future.
func (cache *UploadSelectionCache) refresh(ctx context.Context) (state *uploadselection.State, err error) {
	defer mon.Task()(&ctx)(&err)
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.state != nil && time.Since(cache.lastRefresh) <= cache.staleness {
		return cache.state, nil
	}

	reputableNodes, newNodes, err := cache.db.SelectAllStorageNodesUpload(ctx, cache.selectionConfig)
	if err != nil {
		return cache.state, err
	}

	cache.lastRefresh = time.Now().UTC()
	cache.state = uploadselection.NewState(convSelectedNodesToNodes(reputableNodes), convSelectedNodesToNodes(newNodes))

	mon.IntVal("refresh_cache_size_reputable").Observe(int64(len(reputableNodes)))
	mon.IntVal("refresh_cache_size_new").Observe(int64(len(newNodes)))
	return cache.state, nil
}

// GetNodes selects nodes from the cache that will be used to upload a file.
// Every node selected will be from a distinct network.
// If the cache hasn't been refreshed recently it will do so first.
func (cache *UploadSelectionCache) GetNodes(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

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

	selected, err := state.Select(ctx, uploadselection.Request{
		Count:                req.RequestedCount,
		NewFraction:          cache.selectionConfig.NewNodeFraction,
		Distinct:             cache.selectionConfig.DistinctIP,
		ExcludedIDs:          req.ExcludedIDs,
		Placement:            req.Placement,
		ExcludedCountryCodes: cache.selectionConfig.UploadExcludedCountryCodes,
	})
	if uploadselection.ErrNotEnoughNodes.Has(err) {
		err = ErrNotEnoughNodes.Wrap(err)
	}

	return convNodesToSelectedNodes(selected), err
}

// Size returns how many reputable nodes and new nodes are in the cache.
func (cache *UploadSelectionCache) Size() (reputableNodeCount int, newNodeCount int) {
	cache.mu.RLock()
	state := cache.state
	cache.mu.RUnlock()

	if state == nil {
		return 0, 0
	}

	stats := state.Stats()
	return stats.Reputable, stats.New
}

func convNodesToSelectedNodes(nodes []*uploadselection.Node) (xs []*SelectedNode) {
	for _, n := range nodes {
		xs = append(xs, &SelectedNode{
			ID:          n.ID,
			Address:     &pb.NodeAddress{Address: n.Address},
			LastNet:     n.LastNet,
			LastIPPort:  n.LastIPPort,
			CountryCode: n.CountryCode,
		})
	}
	return xs
}

func convSelectedNodesToNodes(nodes []*SelectedNode) (xs []*uploadselection.Node) {
	for _, n := range nodes {
		xs = append(xs, &uploadselection.Node{
			NodeURL: storj.NodeURL{
				ID:      n.ID,
				Address: n.Address.Address,
			},
			LastNet:     n.LastNet,
			LastIPPort:  n.LastIPPort,
			CountryCode: n.CountryCode,
		})
	}
	return xs
}
