// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uploadselection

import (
	"context"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters.
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// State defines a node selector state that allows for selection.
type State struct {
	mu sync.RWMutex

	stats Stats
	// netByID returns subnet based on storj.NodeID
	netByID map[storj.NodeID]string
	// distinct contains selectors for distinct selection.
	distinct struct {
		Reputable SelectBySubnet
		New       SelectBySubnet
	}
}

// Stats contains state information.
type Stats struct {
	New       int
	Reputable int
}

// Selector defines interface for selecting nodes.
type Selector interface {
	// Count returns the number of maximum number of nodes that it can return.
	Count() int
	// Select selects up-to n nodes which are included by the criteria.
	// empty criteria includes all the nodes
	Select(n int, nodeFilter NodeFilter) []*SelectedNode
}

// NewState returns a state based on the input.
func NewState(reputableNodes, newNodes []*SelectedNode) *State {
	state := &State{}

	state.netByID = map[storj.NodeID]string{}
	for _, node := range reputableNodes {
		state.netByID[node.ID] = node.LastNet
	}
	for _, node := range newNodes {
		state.netByID[node.ID] = node.LastNet
	}

	state.distinct.Reputable = SelectBySubnetFromNodes(reputableNodes)
	state.distinct.New = SelectBySubnetFromNodes(newNodes)

	state.stats = Stats{
		New:       state.distinct.New.Count(),
		Reputable: state.distinct.Reputable.Count(),
	}

	return state
}

// Request contains arguments for State.Request.
type Request struct {
	Count       int
	NewFraction float64
	NodeFilters NodeFilters
}

// Select selects requestedCount nodes where there will be newFraction nodes.
func (state *State) Select(ctx context.Context, request Request) (_ []*SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	state.mu.RLock()
	defer state.mu.RUnlock()

	totalCount := request.Count
	newCount := int(float64(totalCount) * request.NewFraction)

	var selected []*SelectedNode

	var reputableNodes Selector
	var newNodes Selector

	reputableNodes = state.distinct.Reputable
	newNodes = state.distinct.New

	// Get a random selection of new nodes out of the cache first so that if there aren't
	// enough new nodes on the network, we can fall back to using reputable nodes instead.
	selected = append(selected,
		newNodes.Select(newCount, request.NodeFilters)...)

	// Get all the remaining reputable nodes.
	reputableCount := totalCount - len(selected)
	selected = append(selected,
		reputableNodes.Select(reputableCount, request.NodeFilters)...)

	if len(selected) < totalCount {
		return selected, ErrNotEnoughNodes.New("requested from cache %d, found %d", totalCount, len(selected))
	}
	return selected, nil
}

// Stats returns state information.
func (state *State) Stats() Stats {
	state.mu.RLock()
	defer state.mu.RUnlock()

	return state.stats
}

// ExcludeNetworksBasedOnNodes will create a NodeFilter which exclude all nodes which shares subnet with the specified ones.
func (state *State) ExcludeNetworksBasedOnNodes(ds []storj.NodeID) NodeFilter {
	uniqueExcludedNet := make(map[string]struct{}, len(ds))
	for _, id := range ds {
		net := state.netByID[id]
		uniqueExcludedNet[net] = struct{}{}
	}
	excludedNet := make([]string, len(uniqueExcludedNet))
	i := 0
	for net := range uniqueExcludedNet {
		excludedNet[i] = net
		i++
	}
	return ExcludedNetworks(excludedNet)
}
