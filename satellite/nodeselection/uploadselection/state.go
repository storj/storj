// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uploadselection

import (
	"context"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
)

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters.
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// State defines a node selector state that allows for selection.
type State struct {
	mu sync.RWMutex

	stats Stats
	// netByID returns subnet based on storj.NodeID
	netByID map[storj.NodeID]string
	// nonDistinct contains selectors for non-distinct selection.
	nonDistinct struct {
		Reputable SelectByID
		New       SelectByID
	}
	// distinct contains selectors for distinct slection.
	distinct struct {
		Reputable SelectBySubnet
		New       SelectBySubnet
	}
}

// Stats contains state information.
type Stats struct {
	New       int
	Reputable int

	NewDistinct       int
	ReputableDistinct int
}

// Selector defines interface for selecting nodes.
type Selector interface {
	// Count returns the number of maximum number of nodes that it can return.
	Count() int
	// Select selects up-to n nodes which are included by the criteria.
	// empty criteria includes all the nodes
	Select(n int, criteria Criteria) []*Node
}

// NewState returns a state based on the input.
func NewState(reputableNodes, newNodes []*Node) *State {
	state := &State{}

	state.netByID = map[storj.NodeID]string{}
	for _, node := range reputableNodes {
		state.netByID[node.ID] = node.LastNet
	}
	for _, node := range newNodes {
		state.netByID[node.ID] = node.LastNet
	}

	state.nonDistinct.Reputable = SelectByID(reputableNodes)
	state.nonDistinct.New = SelectByID(newNodes)

	state.distinct.Reputable = SelectBySubnetFromNodes(reputableNodes)
	state.distinct.New = SelectBySubnetFromNodes(newNodes)

	state.stats = Stats{
		New:       state.nonDistinct.New.Count(),
		Reputable: state.nonDistinct.Reputable.Count(),

		NewDistinct:       state.distinct.New.Count(),
		ReputableDistinct: state.distinct.Reputable.Count(),
	}

	return state
}

// Request contains arguments for State.Request.
type Request struct {
	Count                int
	NewFraction          float64
	Distinct             bool
	ExcludedIDs          []storj.NodeID
	Placement            storj.PlacementConstraint
	ExcludedCountryCodes []string
}

// Select selects requestedCount nodes where there will be newFraction nodes.
func (state *State) Select(ctx context.Context, request Request) (_ []*Node, err error) {
	defer mon.Task()(&ctx)(&err)

	state.mu.RLock()
	defer state.mu.RUnlock()

	totalCount := request.Count
	newCount := int(float64(totalCount) * request.NewFraction)

	var selected []*Node

	var reputableNodes Selector
	var newNodes Selector

	var criteria Criteria

	if request.ExcludedIDs != nil {
		criteria.ExcludeNodeIDs = request.ExcludedIDs
	}

	for _, code := range request.ExcludedCountryCodes {
		criteria.ExcludedCountryCodes = append(criteria.ExcludedCountryCodes, location.ToCountryCode(code))
	}

	criteria.Placement = request.Placement

	if request.Distinct {
		criteria.AutoExcludeSubnets = make(map[string]struct{})
		for _, id := range request.ExcludedIDs {
			if net, ok := state.netByID[id]; ok {
				criteria.AutoExcludeSubnets[net] = struct{}{}
			}
		}
		reputableNodes = state.distinct.Reputable
		newNodes = state.distinct.New
	} else {
		reputableNodes = state.nonDistinct.Reputable
		newNodes = state.nonDistinct.New
	}

	// Get a random selection of new nodes out of the cache first so that if there aren't
	// enough new nodes on the network, we can fall back to using reputable nodes instead.
	selected = append(selected,
		newNodes.Select(newCount, criteria)...)

	// Get all the remaining reputable nodes.
	reputableCount := totalCount - len(selected)
	selected = append(selected,
		reputableNodes.Select(reputableCount, criteria)...)

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
