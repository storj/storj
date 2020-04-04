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
// The info we need about a node in the cache is the minimum info
// to create an order limit
type CachedNode struct {
	ID             storj.NodeID
	LastNet        string
	LastIPPort     string
	MinimumVersion string // semver or empty
}

// SelectedNodesCache keeps a list of all the storage nodes qualified to store data
type SelectedNodesCache struct {
	log              *zap.Logger
	db               DB
	vetted           []CachedNode
	vettedMu         sync.Mutex
	unvetted         []CachedNode
	unvettedMu       sync.Mutex
	nodeSelectionCfg NodeSelectionConfig
}

// NewSelectedNodesCache is creates a new cache
func NewSelectedNodesCache(ctx context.Context, log *zap.Logger, db DB, cfg NodeSelectionConfig) *SelectedNodesCache {
	return &SelectedNodesCache{
		log:              log,
		db:               db,
		vetted:           []CachedNode{},
		unvetted:         []CachedNode{},
		nodeSelectionCfg: cfg,
	}
}

// Init populates the cache with all the vetted and unvetted nodes
// from the nodes table in the overlay database
func (c *SelectedNodesCache) Init(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	allVetted, err := c.db.SelectAllVettedStorageNodes(ctx, c.nodeSelectionCfg)
	if err != nil {
		return err
	}
	c.vettedMu.Lock()
	c.vetted = allVetted
	c.vettedMu.Unlock()

	allUnvetted, err := c.db.SelectAllUnvettedStorageNodes(ctx, c.nodeSelectionCfg)
	if err != nil {
		return err
	}
	c.unvettedMu.Lock()
	c.unvetted = allUnvetted
	c.unvettedMu.Unlock()
	return nil
}

// GetNodes selects nodes from the cache. Every node selected will be from a distinct network.
func (c *SelectedNodesCache) GetNodes(ctx context.Context, req FindStorageNodesRequest) (_ []CachedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	// calculate how many vetted versus unvetted nodes are needed
	totalcount := req.RequestedCount
	unvettedcount := float64(req.RequestedCount) * c.nodeSelectionCfg.NewNodeFraction
	vettedcount := totalcount - int(unvettedcount)

	var selectedNodeResults = []CachedNode{}
	var distinctNetworks = map[string]struct{}{}
	var callCount int
	const maxDepth = 100
	// we need to select vettedcount number of nodes from the cache.
	// however we want to make sure that the nodes are from distinct networks
	// and also not in the excluded list. We keep looping to find more nodes
	// until we have vettedcount number of nodes that fulfil that criteria.
	for len(selectedNodeResults) < vettedcount {
		if callCount > maxDepth {
			return nil, Error.New("unable to select enough vetted nodes")
		}
		callCount++
		// randomly select vettedcount number of nodes from the cache
		randomIdexes := rand.Perm(vettedcount)
		for _, vettedIdx := range randomIdexes {
			currNode := c.vetted[vettedIdx]
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
			if len(selectedNodeResults) >= vettedcount {
				break
			}
		}
	}

	callCount = 0
	for len(selectedNodeResults) < vettedcount+int(unvettedcount) {
		if callCount > maxDepth {
			return nil, Error.New("unable to select enough unvetted nodes")
		}
		callCount++
		randomIdexes := rand.Perm(int(unvettedcount))
		for _, unvettedIdx := range randomIdexes {
			currNode := c.unvetted[unvettedIdx]
			if _, ok := distinctNetworks[currNode.LastNet]; ok {
				continue
			}
			if _, ok := req.ExcludedIDsMap[currNode.ID]; ok {
				continue
			}

			selectedNodeResults = append(selectedNodeResults, currNode)
			distinctNetworks[currNode.LastNet] = struct{}{}
			if len(selectedNodeResults) >= vettedcount+int(unvettedcount) {
				break
			}
		}
	}

	return selectedNodeResults, nil
}

// RemoveVetted removes a vetted node from the selected nodes cache
func (c *SelectedNodesCache) RemoveVetted(ctx context.Context, nodeID storj.NodeID) {
	defer mon.Task()(&ctx)(nil)

	for i, node := range c.vetted {
		if node.ID == nodeID {
			c.vetted = append(c.vetted[:i], c.vetted[:i+1]...)
			// could there be more than 1 node in the cache with this id?
			// should we keep a map of node ids that keeps a count of how many are in the cache with that id
			return
		}
	}
	// should we log here that the nodeID was not found?
	return
}

// AddVetted adds a vetted node to the selected nodes cache
func (c *SelectedNodesCache) AddVetted(ctx context.Context, node CachedNode) {
	defer mon.Task()(&ctx)(nil)

	c.vettedMu.Lock()
	c.vetted = append(c.vetted, node)
	c.vettedMu.Unlock()
}

// RemoveUnvetted removes a unvetted node from the selected nodes cache
func (c *SelectedNodesCache) RemoveUnvetted(ctx context.Context, nodeID storj.NodeID) {
	defer mon.Task()(&ctx)(nil)

	for i, node := range c.unvetted {
		if node.ID == nodeID {
			c.unvetted = append(c.unvetted[:i], c.unvetted[:i+1]...)
			return
		}
	}
}

// AddUnvetted adds a unvetted node to the selected nodes cache
func (c *SelectedNodesCache) AddUnvetted(ctx context.Context, node CachedNode) {
	defer mon.Task()(&ctx)(nil)

	c.unvettedMu.Lock()
	c.unvetted = append(c.unvetted, node)
	c.unvettedMu.Unlock()
}
