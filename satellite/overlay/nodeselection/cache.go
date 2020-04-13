// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// Config is a configuration for overlay nodeselection cache.
type Config struct {
	CacheStaleness time.Duration `help:"how stale the node selection cache can be" releaseDefault:"3m" devDefault:"5m"`
}

// DB implements the database for overlay.Nodeselection
//
// architecture: Database
type DB interface {
	// SelectAllStorageNodesUpload returns all nodes that qualify to store data, organized as reputable nodes and new nodes
	SelectAllStorageNodesUpload(ctx context.Context, selectionCfg overlay.NodeSelectionConfig) (reputable, new []CachedNode, err error)
}

// Cache keeps a list of all the storage nodes that are qualified to store data
// We organize the nodes by if they are reputable or a new node on the network.
// The cache will get refreshed once the staleness time has past.
type Cache struct {
	log              *zap.Logger
	db               DB
	nodeSelectionCfg overlay.NodeSelectionConfig
	staleness        time.Duration

	mu             sync.Mutex
	reputableNodes []CachedNode
	newNodes       []CachedNode
	lastRefresh    time.Time
}

// CachedNode contains all the info about a node in the cache
// The info we need about a node in the cache is info to create an order limit
type CachedNode struct {
	ID         storj.NodeID
	Address    string
	LastNet    string
	LastIPPort string
}

// NewCache creates a new cache that keeps a list of all the storage nodes that are qualified to store data
func NewCache(ctx context.Context, log *zap.Logger, db DB, staleness time.Duration, cfg overlay.NodeSelectionConfig) *Cache {
	return &Cache{
		log:              log,
		db:               db,
		staleness:        staleness,
		nodeSelectionCfg: cfg,
		reputableNodes:   []CachedNode{},
		newNodes:         []CachedNode{},
	}
}

// Init populates the cache with all of the reputableNodes and newNode nodes
// that qualify to upload data from the nodes table in the overlay database
func (c *Cache) Init(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return c.Refresh(ctx)
}

// Refresh calls out to the database and refreshes the cache with the current data
// from the nodes table, then sets time that the last refresh occurred so we know when
// to refresh again in the future
func (c *Cache) Refresh(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	reputableNodes, newNodes, err := c.db.SelectAllStorageNodesUpload(ctx, c.nodeSelectionCfg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.reputableNodes = reputableNodes
	c.newNodes = newNodes
	c.SetLastRefresh(ctx, time.Now().UTC())
	c.mu.Unlock()

	mon.IntVal("refresh_cache_size_reputable").Observe(int64(len(c.reputableNodes)))
	mon.IntVal("refresh_cache_size_new").Observe(int64(len(c.newNodes)))
	return nil
}

// SetLastRefresh stores when the last refresh occurred
func (c *Cache) SetLastRefresh(ctx context.Context, lastRefresh time.Time) {
	defer mon.Task()(&ctx)(nil)
	c.lastRefresh = lastRefresh
}

// GetNodes selects nodes from the cache that will be used to upload a file.
// Every node selected will be from a distinct network.
// If the cache has no been refreshed recently, then refresh first.
func (c *Cache) GetNodes(ctx context.Context, req overlay.FindStorageNodesRequest) (_ []CachedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	if time.Since(c.lastRefresh) > c.staleness {
		err := c.Refresh(ctx)
		if err != nil {
			return nil, err
		}
	}

	// how many reputableNodes versus newNode nodes are needed
	totalcount := req.RequestedCount
	newNodeCount := int(float64(req.RequestedCount) * c.nodeSelectionCfg.NewNodeFraction)
	reputableNodeCount := totalcount - newNodeCount

	var selectedNodeResults = []CachedNode{}
	var distinctNetworks = map[string]struct{}{}

	// randomly select nodes from the cache
	randomIdexes := rand.Perm(len(c.reputableNodes))
	for _, idx := range randomIdexes {
		currNode := c.reputableNodes[idx]

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
		if len(selectedNodeResults) >= reputableNodeCount {
			break
		}
	}

	randomIdexes = rand.Perm(len(c.newNodes))
	for _, idx := range randomIdexes {
		currNode := c.newNodes[idx]
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
		if len(selectedNodeResults) >= reputableNodeCount+newNodeCount {
			break
		}
	}
	if len(selectedNodeResults) < reputableNodeCount+newNodeCount {
		c.log.Error("not enough nodes for a selection from the selected nodes cache",
			zap.Int("needed", reputableNodeCount+newNodeCount),
			zap.Int("actual", len(selectedNodeResults)),
		)
	}

	return selectedNodeResults, nil
}

// Size returns the size of the reputable nodes and new nodes in the cache
func (c *Cache) Size(ctx context.Context) (reputableNodeCount int, newNodeCount int) {
	return len(c.reputableNodes), len(c.newNodes)
}
