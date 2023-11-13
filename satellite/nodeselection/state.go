// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// AllowSameSubnet is a short to check if Subnet exclusion is disabled == allow pick nodes from the same subnet.
func AllowSameSubnet(filter NodeFilter) bool {
	return GetAnnotation(filter, AutoExcludeSubnet) == AutoExcludeSubnetOFF
}

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters.
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// State defines a node selector state that allows for selection.
type State struct {
	mu sync.RWMutex

	// netByID returns subnet based on storj.NodeID
	netByID map[storj.NodeID]string

	// byNetwork contains selectors for distinct selection.
	byNetwork struct {
		Reputable SelectBySubnet
		New       SelectBySubnet
	}

	byID struct {
		Reputable SelectByID
		New       SelectByID
	}
}

// Selector defines interface for selecting nodes.
type Selector interface {
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

	state.byNetwork.Reputable = SelectBySubnetFromNodes(reputableNodes)
	state.byNetwork.New = SelectBySubnetFromNodes(newNodes)

	state.byID.Reputable = SelectByID(reputableNodes)
	state.byID.New = SelectByID(newNodes)

	return state
}

// SelectionType defines how to select nodes randomly.
type SelectionType int8

const (
	// SelectionTypeByNetwork chooses subnets randomly, and one node from each subnet.
	SelectionTypeByNetwork = iota

	// SelectionTypeByID chooses nodes randomly.
	SelectionTypeByID
)

// Request contains arguments for State.Request.
type Request struct {
	Count         int
	NewFraction   float64
	NodeFilters   NodeFilters
	SelectionType SelectionType
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

	switch request.SelectionType {
	case SelectionTypeByNetwork:
		reputableNodes = state.byNetwork.Reputable
		newNodes = state.byNetwork.New
	case SelectionTypeByID:
		reputableNodes = state.byID.Reputable
		newNodes = state.byID.New
	default:
		return nil, errs.New("Unsupported selection type: %d", request.SelectionType)
	}

	// Get a random selection of new nodes out of the cache first so that if there aren't
	// enough new nodes on the network, we can fall back to using reputable nodes instead.
	selected = append(selected,
		newNodes.Select(newCount, request.NodeFilters)...)

	// Get all the remaining reputable nodes.
	reputableCount := totalCount - len(selected)

	filters := request.NodeFilters
	if GetAnnotation(filters, AutoExcludeSubnet) != AutoExcludeSubnetOFF {
		filters = append(append(NodeFilters{}, request.NodeFilters...), ExcludedNodeNetworks(selected))
	}

	selected = append(selected, reputableNodes.Select(reputableCount, filters)...)

	if len(selected) < totalCount {
		return selected, ErrNotEnoughNodes.New("requested from cache %d, found %d", totalCount, len(selected))
	}
	return selected, nil
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
