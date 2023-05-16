// Copyright (C) 2019 Storj Labs, Incache.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
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

	cache sync2.ReadCacheOf[*uploadselection.State]
}

// NewUploadSelectionCache creates a new cache that keeps a list of all the storage nodes that are qualified to store data.
func NewUploadSelectionCache(log *zap.Logger, db UploadSelectionDB, staleness time.Duration, config NodeSelectionConfig) (*UploadSelectionCache, error) {
	cache := &UploadSelectionCache{
		log:             log,
		db:              db,
		selectionConfig: config,
	}
	return cache, cache.cache.Init(staleness/2, staleness, cache.read)
}

// Run runs the background task for cache.
func (cache *UploadSelectionCache) Run(ctx context.Context) (err error) {
	return cache.cache.Run(ctx)
}

// Refresh populates the cache with all of the reputableNodes and newNode nodes
// This method is useful for tests.
func (cache *UploadSelectionCache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = cache.cache.RefreshAndGet(ctx, time.Now())
	return err
}

// refresh calls out to the database and refreshes the cache with the most up-to-date
// data from the nodes table, then sets time that the last refresh occurred so we know when
// to refresh again in the future.
func (cache *UploadSelectionCache) read(ctx context.Context) (_ *uploadselection.State, err error) {
	defer mon.Task()(&ctx)(&err)

	reputableNodes, newNodes, err := cache.db.SelectAllStorageNodesUpload(ctx, cache.selectionConfig)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	state := uploadselection.NewState(convSelectedNodesToNodes(reputableNodes), convSelectedNodesToNodes(newNodes))

	mon.IntVal("refresh_cache_size_reputable").Observe(int64(len(reputableNodes)))
	mon.IntVal("refresh_cache_size_new").Observe(int64(len(newNodes)))

	return state, nil
}

// GetNodes selects nodes from the cache that will be used to upload a file.
// Every node selected will be from a distinct network.
// If the cache hasn't been refreshed recently it will do so first.
func (cache *UploadSelectionCache) GetNodes(ctx context.Context, req FindStorageNodesRequest) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	state, err := cache.cache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	selected, err := state.Select(ctx, uploadselection.Request{
		Count:                req.RequestedCount,
		NewFraction:          cache.selectionConfig.NewNodeFraction,
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
func (cache *UploadSelectionCache) Size(ctx context.Context) (reputableNodeCount int, newNodeCount int, _ error) {
	state, err := cache.cache.Get(ctx, time.Now())
	if err != nil {
		return 0, 0, Error.Wrap(err)
	}
	stats := state.Stats()
	return stats.Reputable, stats.New, nil
}

// GetNodesNetwork returns the cached network for each given node ID.
func (cache *UploadSelectionCache) GetNodesNetwork(ctx context.Context, nodeIDs []storj.NodeID) (nets []string, err error) {
	state, err := cache.cache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return state.GetNodesNetwork(ctx, nodeIDs), nil
}

func convNodesToSelectedNodes(nodes []*uploadselection.Node) (xs []*SelectedNode) {
	for _, n := range nodes {
		xs = append(xs, &SelectedNode{
			ID:          n.ID,
			Address:     pb.NodeFromNodeURL(n.NodeURL).Address,
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
			NodeURL: (&pb.Node{
				Id:      n.ID,
				Address: n.Address,
			}).NodeURL(),
			LastNet:     n.LastNet,
			LastIPPort:  n.LastIPPort,
			CountryCode: n.CountryCode,
		})
	}
	return xs
}
