// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"storj.io/common/storj"
)

// SelectedNodesCache keeps a list of all the storage nodes that are qualified to store data
// We organize the nodes by if they are reputable or a new node on the network.
type SelectedNodesCache struct {
	log              *zap.Logger
	db               DB
	nodeSelectionCfg NodeSelectionConfig

	reputableMu    sync.Mutex
	reputableNodes []CachedNode

	newNodesMu sync.Mutex
	newNodes   []CachedNode
}

// CachedNode contains all the info about a node in the cache
// The info we need about a node in the cache is info to create an order limit
type CachedNode struct {
	ID         storj.NodeID
	Address    string
	LastNet    string
	LastIPPort string
}

// NewSelectedNodesCache creates a new cache that keeps a list of all the storage nodes that are qualified to store data
func NewSelectedNodesCache(ctx context.Context, log *zap.Logger, db DB, cfg NodeSelectionConfig) *SelectedNodesCache {
	rand.Seed(time.Now().UnixNano())
	return &SelectedNodesCache{
		log:              log,
		db:               db,
		nodeSelectionCfg: cfg,
		reputableNodes:   []CachedNode{},
		newNodes:         []CachedNode{},
	}
}

// Init populates the cache with all of the reputableNodes and newNode nodes
// that qualify to upload data from the nodes table in the overlay database
func (c *SelectedNodesCache) Init(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	reputableNodes, newNodes, err := c.db.SelectAllStorageNodesUpload(ctx, c.nodeSelectionCfg)
	if err != nil {
		return err
	}
	c.reputableMu.Lock()
	c.reputableNodes = reputableNodes
	c.reputableMu.Unlock()

	c.newNodesMu.Lock()
	c.newNodes = newNodes
	c.newNodesMu.Unlock()

	mon.IntVal("selected_nodes_cache_reputable_size").Observe(int64(len(reputableNodes)))
	mon.IntVal("selected_nodes_cache_new_size").Observe(int64(len(newNodes)))
	return nil
}

// GetNodes selects nodes from the cache that will be used to upload a file.
// Every node selected will be from a distinct network.
func (c *SelectedNodesCache) GetNodes(ctx context.Context, req FindStorageNodesRequest) (_ []CachedNode, err error) {
	defer mon.Task()(&ctx)(&err)

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
		if _, ok := req.ExcludedIDsMap[currNode.ID]; ok {
			continue
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
		if _, ok := req.ExcludedIDsMap[currNode.ID]; ok {
			continue
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

// RemoveNode removes a node from the selected nodes cache
func (c *SelectedNodesCache) RemoveNode(ctx context.Context, nodeID storj.NodeID) {
	defer mon.Task()(&ctx)(nil)

	for i, node := range c.reputableNodes {
		if node.ID == nodeID {
			c.reputableMu.Lock()
			c.reputableNodes[len(c.reputableNodes)-1], c.reputableNodes[i] = c.reputableNodes[i], c.reputableNodes[len(c.reputableNodes)-1]
			c.reputableNodes = c.reputableNodes[:len(c.reputableNodes)-1]
			c.reputableMu.Unlock()
			mon.IntVal("remove_reputable_new_cache_size").Observe(int64(len(c.reputableNodes)))
			return
		}
	}

	for i, node := range c.newNodes {
		if node.ID == nodeID {
			c.newNodesMu.Lock()
			c.newNodes[len(c.newNodes)-1], c.newNodes[i] = c.newNodes[i], c.newNodes[len(c.newNodes)-1]
			c.newNodes = c.newNodes[:len(c.newNodes)-1]
			c.newNodesMu.Unlock()
			mon.IntVal("remove_new__node_new_cache_size").Observe(int64(len(c.newNodes)))
			return
		}
	}
	c.log.Debug("nodeID not found in cache", zap.String("node id", nodeID.String()))
	return
}

// AddReputableNode adds a reputable node to the selected nodes cache
func (c *SelectedNodesCache) AddReputableNode(ctx context.Context, node CachedNode) {
	defer mon.Task()(&ctx)(nil)

	c.reputableMu.Lock()
	c.reputableNodes = append(c.reputableNodes, node)
	c.reputableMu.Unlock()
	mon.IntVal("added_reputable_new_cache_size").Observe(int64(len(c.reputableNodes)))
}

// AddNewNode adds a new node to the selected nodes cache
func (c *SelectedNodesCache) AddNewNode(ctx context.Context, node CachedNode) {
	defer mon.Task()(&ctx)(nil)

	c.newNodesMu.Lock()
	c.newNodes = append(c.newNodes, node)
	c.newNodesMu.Unlock()
	mon.IntVal("added_new_node_new_cache_size").Observe(int64(len(c.newNodes)))
}

// Size returns the size of the reputable nodes and new nodes in the cache
func (c *SelectedNodesCache) Size(ctx context.Context) (int, int) {
	return len(c.reputableNodes), len(c.newNodes)
}
