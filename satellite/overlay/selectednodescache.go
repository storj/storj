// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"math/rand"
	"sync"

	"go.uber.org/zap"
	"storj.io/common/storj"
)

// CachedNode contains all the info about a node in the cache
// The info we need about a node in the cache is info to create an order limit
type CachedNode struct {
	ID         storj.NodeID
	Address    string
	LastNet    string
	LastIPPort string
}

// SelectedNodesCache keeps a list of all the storage nodes that are qualified to store data
// We organize the nodes by if they are reputable or new nodes.
type SelectedNodesCache struct {
	log              *zap.Logger
	db               DB
	nodeSelectionCfg NodeSelectionConfig

	reputableMu    sync.Mutex
	reputableNodes []CachedNode

	newNodeMu sync.Mutex
	newNode   []CachedNode
}

// NewSelectedNodesCache creates a new cache that keeps a list of all the storage nodes that are qualified to store data
func NewSelectedNodesCache(ctx context.Context, log *zap.Logger, db DB, cfg NodeSelectionConfig) *SelectedNodesCache {
	return &SelectedNodesCache{
		log:              log,
		db:               db,
		nodeSelectionCfg: cfg,
		reputableNodes:   []CachedNode{},
		newNode:          []CachedNode{},
	}
}

// Init populates the cache with all the reputableNodes and newNode nodes
// from the nodes table in the overlay database
func (c *SelectedNodesCache) Init(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	reputableNodes, newNodes, err := c.db.SelectAllStorageNodes(ctx, c.nodeSelectionCfg)
	if err != nil {
		return err
	}
	c.reputableMu.Lock()
	c.reputableNodes = reputableNodes
	c.reputableMu.Unlock()

	c.newNodeMu.Lock()
	c.newNode = newNodes
	c.newNodeMu.Unlock()
	return nil
}

// GetNodes selects nodes from the cache. Every node selected will be from a distinct network.
func (c *SelectedNodesCache) GetNodes(ctx context.Context, req FindStorageNodesRequest) (_ []CachedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	// calculate how many reputableNodes versus newNode nodes are needed
	totalcount := req.RequestedCount
	newNodecount := float64(req.RequestedCount) * c.nodeSelectionCfg.NewNodeFraction
	reputableNodescount := totalcount - int(newNodecount)

	var selectedNodeResults = []CachedNode{}
	var distinctNetworks = map[string]struct{}{}
	var callCount int
	const maxDepth = 100
	// we need to select reputableNodescount number of nodes from the cache.
	// however we want to make sure that the nodes are from distinct networks
	// and also not in the excluded list. We keep looping to find more nodes
	// until we have reputableNodescount number of nodes that fulfil that criteria.
	for len(selectedNodeResults) < reputableNodescount {
		if callCount > maxDepth {
			return nil, Error.New("unable to select enough reputableNodes nodes")
		}
		callCount++
		// randomly select reputableNodescount number of nodes from the cache
		randomIdexes := rand.Perm(len(c.reputableNodes))
		for _, reputableNodesIdx := range randomIdexes {
			currNode := c.reputableNodes[reputableNodesIdx]
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
			if len(selectedNodeResults) >= reputableNodescount {
				break
			}
		}
	}

	callCount = 0
	for len(selectedNodeResults) < reputableNodescount+int(newNodecount) {
		if callCount > maxDepth {
			return nil, Error.New("unable to select enough newNode nodes")
		}
		callCount++
		randomIdexes := rand.Perm(len(c.newNode))
		for _, newNodeIdx := range randomIdexes {
			currNode := c.newNode[newNodeIdx]
			if _, ok := distinctNetworks[currNode.LastNet]; ok {
				continue
			}
			if _, ok := req.ExcludedIDsMap[currNode.ID]; ok {
				continue
			}

			selectedNodeResults = append(selectedNodeResults, currNode)
			distinctNetworks[currNode.LastNet] = struct{}{}
			if len(selectedNodeResults) >= reputableNodescount+int(newNodecount) {
				break
			}
		}
	}

	return selectedNodeResults, nil
}

// RemoveNode removes a node from the selected nodes cache
func (c *SelectedNodesCache) RemoveNode(ctx context.Context, nodeID storj.NodeID) {
	defer mon.Task()(&ctx)(nil)

	for i, node := range c.reputableNodes {
		if node.ID == nodeID {
			c.reputableMu.Lock()
			c.reputableNodes = append(c.reputableNodes[:i], c.reputableNodes[:i+1]...)
			c.reputableMu.Unlock()
			// could there be more than 1 node in the cache with this id?
			// should we keep a map of node ids that keeps a count of how many are in the cache with that id
			return
		}
	}

	for i, node := range c.newNode {
		if node.ID == nodeID {
			c.newNodeMu.Lock()
			c.newNode = append(c.newNode[:i], c.newNode[:i+1]...)
			c.newNodeMu.Unlock()
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
}

// AddNewNode adds a new node to the selected nodes cache
func (c *SelectedNodesCache) AddNewNode(ctx context.Context, node CachedNode) {
	defer mon.Task()(&ctx)(nil)

	c.newNodeMu.Lock()
	c.newNode = append(c.newNode, node)
	c.newNodeMu.Unlock()
}
