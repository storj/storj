// Copyright (C) 2019 Storj Labs, Incache.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/nodeselection"
)

// UploadSelectionDB implements the database for upload selection cache.
//
// architecture: Database
type UploadSelectionDB interface {
	// SelectAllStorageNodesUpload returns all nodes that qualify to store data, organized as reputable nodes and new nodes
	SelectAllStorageNodesUpload(ctx context.Context, selectionCfg NodeSelectionConfig) (reputable, new []*nodeselection.SelectedNode, err error)
}

// UploadSelectionCacheConfig is a configuration for upload selection cache.
type UploadSelectionCacheConfig struct {
	Disabled  bool          `help:"disable node cache" default:"false" deprecated:"true"`
	Staleness time.Duration `help:"how stale the node selection cache can be" releaseDefault:"3m" devDefault:"5m" testDefault:"3m"`
}

// UploadSelectionCache keeps a list of all the storage nodes that are qualified to store data
// We organize the nodes by if they are reputable or a new node on the network.
// The cache will sync with the nodes table in the database and get refreshed once the staleness time has past.
type UploadSelectionCache struct {
	log             *zap.Logger
	db              UploadSelectionDB
	selectionConfig NodeSelectionConfig

	cache sync2.ReadCacheOf[nodeselection.State]

	defaultFilters nodeselection.NodeFilters
	placements     nodeselection.PlacementDefinitions
}

// NewUploadSelectionCache creates a new cache that keeps a list of all the storage nodes that are qualified to store data.
func NewUploadSelectionCache(log *zap.Logger, db UploadSelectionDB, staleness time.Duration, config NodeSelectionConfig, defaultFilter nodeselection.NodeFilters, placements nodeselection.PlacementDefinitions) (*UploadSelectionCache, error) {
	cache := &UploadSelectionCache{
		log:             log,
		db:              db,
		selectionConfig: config,
		defaultFilters:  defaultFilter,
		placements:      placements,
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
func (cache *UploadSelectionCache) read(ctx context.Context) (_ nodeselection.State, err error) {
	defer mon.Task()(&ctx)(&err)

	reputableNodes, newNodes, err := cache.db.SelectAllStorageNodesUpload(ctx, cache.selectionConfig)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var allNodes = append(append([]*nodeselection.SelectedNode{}, reputableNodes...), newNodes...)
	reportMetrics(allNodes, cache.placements)
	state := nodeselection.InitState(ctx, allNodes, cache.placements)
	return state, nil
}

// PlacementMetrics is a struct that holds the metrics for a specific placement.
// This is a workaround, as all other IntVal/FloatVal/etc registers too many unnecessary fields.
type PlacementMetrics struct {
	UploadCount    float64
	Count          float64
	UploadFreeDisk float64
}

func reportMetrics(nodes []*nodeselection.SelectedNode, placements nodeselection.PlacementDefinitions) {
	reputable := 0
	count := map[storj.PlacementConstraint]int64{}
	uploadCount := map[storj.PlacementConstraint]int64{}
	uploadFreeDisk := map[storj.PlacementConstraint]int64{}
	for _, node := range nodes {
		if node.Vetted {
			reputable++
		}
		for _, placement := range placements {
			if placement.NodeFilter == nil || placement.NodeFilter.Match(node) {
				count[placement.ID]++
				if placement.UploadFilter == nil || placement.UploadFilter.Match(node) {
					uploadCount[placement.ID]++
					uploadFreeDisk[placement.ID] += node.FreeDisk
				}
			}
		}
	}

	mon.IntVal("refresh_cache_size_reputable").Observe(int64(reputable))
	mon.IntVal("refresh_cache_size_new").Observe(int64(len(nodes) - reputable))

	for _, placement := range placements {
		mon.StructVal("placement",
			monkit.NewSeriesTag("name", placement.Name),
			monkit.NewSeriesTag("id", fmt.Sprintf("%d", placement.ID))).
			Observe(PlacementMetrics{
				UploadCount:    float64(uploadCount[placement.ID]),
				Count:          float64(count[placement.ID]),
				UploadFreeDisk: float64(uploadFreeDisk[placement.ID]),
			})
	}
}

// GetNodes selects nodes from the cache that will be used to upload a file.
// Every node selected will be from a distinct network.
// If the cache hasn't been refreshed recently it will do so first.
func (cache *UploadSelectionCache) GetNodes(ctx context.Context, req FindStorageNodesRequest) (_ []*nodeselection.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	state, err := cache.cache.Get(ctx, time.Now())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	nodes, err := state.Select(ctx, req.Requester, req.Placement, req.RequestedCount, req.ExcludedIDs, req.AlreadySelected)
	if nodeselection.ErrNotEnoughNodes.Has(err) {
		err = ErrNotEnoughNodes.Wrap(err)
	}
	return nodes, err
}
