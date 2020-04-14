// Copyright (C) 2019 Storj Labs, Incache.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CacheDB implements the database for overlay node selection cache
//
// architecture: Database
type CacheDB interface {
	// SelectAllStorageNodesUpload returns all nodes that qualify to store data, organized as reputable nodes and new nodes
	SelectAllStorageNodesUpload(ctx context.Context, selectionCfg NodeSelectionConfig) (reputable, new []*SelectedNode, err error)
}

// CacheConfig is a configuration for overlay node selection cache.
type CacheConfig struct {
	Staleness time.Duration `help:"how stale the node selection cache can be" releaseDefault:"3m" devDefault:"5m"`
}

// NodeSelectionCache keeps a list of all the storage nodes that are qualified to store data
// We organize the nodes by if they are reputable or a new node on the network.
// The cache will get refreshed once the staleness time has past.
type NodeSelectionCache struct {
	log             *zap.Logger
	db              CacheDB
	selectionConfig NodeSelectionConfig
	staleness       time.Duration

	mu   sync.RWMutex
	data *state
}

type state struct {
	lastRefresh time.Time

	mu             sync.RWMutex
	reputableNodes []*SelectedNode
	newNodes       []*SelectedNode
}

// NewNodeSelectionCache creates a new cache that keeps a list of all the storage nodes that are qualified to store data
func NewNodeSelectionCache(log *zap.Logger, db CacheDB, staleness time.Duration, config NodeSelectionConfig) *NodeSelectionCache {
	return &NodeSelectionCache{
		log:             log,
		db:              db,
		staleness:       staleness,
		selectionConfig: config,
		data:            &state{},
	}
}

// Init populates the cache with all of the reputableNodes and newNode nodes
// that qualify to upload data from the nodes table in the overlay database
func (cache *NodeSelectionCache) Init(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	_, err = cache.refresh(ctx)
	return err
}

// Refresh calls out to the database and refreshes the cache with the current data
// from the nodes table, then sets time that the last refresh occurred so we know when
// to refresh again in the future
func (cache *NodeSelectionCache) refresh(ctx context.Context) (cachData *state, err error) {
	defer mon.Task()(&ctx)(&err)

	if cache.data != nil && time.Since(cache.data.lastRefresh) <= cache.staleness {
		return cache.data, nil
	}

	reputableNodes, newNodes, err := cache.db.SelectAllStorageNodesUpload(ctx, cache.selectionConfig)
	if err != nil {
		return cache.data, err
	}
	cache.data = &state{
		lastRefresh:    time.Now().UTC(),
		reputableNodes: reputableNodes,
		newNodes:       newNodes,
	}

	mon.IntVal("refresh_cache_size_reputable").Observe(int64(len(reputableNodes)))
	mon.IntVal("refresh_cache_size_new").Observe(int64(len(newNodes)))
	return cache.data, nil
}

// GetNodes selects nodes from the cache that will be used to upload a file.
// Every node selected will be from a distinct network.
// If the cache has no been refreshed recently, then refresh first.
func (cache *NodeSelectionCache) GetNodes(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	cache.mu.RLock()
	cacheData := cache.data
	cache.mu.RUnlock()

	// if the cache is stale, then refresh it before we get nodes
	if time.Since(cacheData.lastRefresh) > cache.staleness {
		cacheData, err = cache.refresh(ctx)
		if err != nil {
			return nil, err
		}
	}

	return cacheData.GetNodes(ctx, req, cache.selectionConfig.NewNodeFraction)
}

// GetNodes selects nodes from the cache that will be used to upload a file.
// Every node selected will be from a distinct network.
// If the cache has no been refreshed recently, then refresh first.
func (cacheData *state) GetNodes(ctx context.Context, req FindStorageNodesRequest, newNodeFraction float64) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	cacheData.mu.RLock()
	defer cacheData.mu.RUnlock()

	// how many reputableNodes versus newNode nodes are needed
	totalcount := req.RequestedCount
	newNodeCount := int(float64(req.RequestedCount) * newNodeFraction)

	var selectedNodeResults = []*SelectedNode{}
	var distinctNetworks = map[string]struct{}{}

	// randomly select nodes from the cache
	// select new nodes first so that if there aren't enough new nodes
	// on the network, we can fall back to using reputable nodes instead
	randomIdexes := rand.Perm(len(cacheData.newNodes))
	for _, idx := range randomIdexes {
		currNode := cacheData.newNodes[idx]
		if _, ok := distinctNetworks[currNode.LastNet]; ok {
			continue
		}
		for _, excludedID := range req.ExcludedIDs {
			if excludedID == currNode.ID {
				continue
			}
		}

		selectedNodeResults = append(selectedNodeResults, currNode)
		distinctNetworks[currNode.LastNet] = struct{}{}
		if len(selectedNodeResults) >= newNodeCount {
			break
		}
	}

	randomIdexes = rand.Perm(len(cacheData.reputableNodes))
	for _, idx := range randomIdexes {
		currNode := cacheData.reputableNodes[idx]

		// don't select a node if we've already selected another node from the same network
		if _, ok := distinctNetworks[currNode.LastNet]; ok {
			continue
		}
		// don't select a node listed in the excluded list
		for _, excludedID := range req.ExcludedIDs {
			if excludedID == currNode.ID {
				continue
			}
		}

		selectedNodeResults = append(selectedNodeResults, currNode)
		distinctNetworks[currNode.LastNet] = struct{}{}
		if len(selectedNodeResults) >= totalcount {
			break
		}
	}

	if len(selectedNodeResults) < totalcount {
		return nil, Error.New("unable to select enough nodes from node selection cache, needed: %d, actual: %d",
			totalcount, len(selectedNodeResults),
		)
	}
	return selectedNodeResults, nil
}

// Size returns the size of the reputable nodes and new nodes in the cache
func (cache *NodeSelectionCache) Size() (reputableNodeCount int, newNodeCount int) {
	cache.mu.RLock()
	cacheData := cache.data
	cache.mu.RUnlock()
	return cacheData.size()
}

func (cacheData *state) size() (reputableNodeCount int, newNodeCount int) {
	cacheData.mu.RLock()
	defer cacheData.mu.RUnlock()
	return len(cacheData.reputableNodes), len(cacheData.newNodes)
}
