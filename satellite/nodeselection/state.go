// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ErrNotEnoughNodes is when selecting nodes failed with the given parameters.
var ErrNotEnoughNodes = errs.Class("not enough nodes")

// State includes a stateful selector (indexed nodes) for each placement.
type State map[storj.PlacementConstraint]NodeSelector

var initStateTask = mon.Task()

// InitState initializes the State for each placement.
func InitState(ctx context.Context, nodes []*SelectedNode, placements PlacementDefinitions) State {
	defer initStateTask(&ctx)(nil)
	state := make(State)
	for id, placement := range placements {
		selector := placement.Selector
		if selector == nil {
			selector = RandomSelector()
		}
		var filter = placement.NodeFilter
		if placement.UploadFilter != nil {
			filter = NodeFilters{placement.NodeFilter, placement.UploadFilter}
		}
		state[id] = selector(ctx, nodes, filter)
	}
	return state
}

var selectTask = mon.Task()

// Select picks the required nodes given a specific placement.
func (s State) Select(ctx context.Context, requester storj.NodeID, p storj.PlacementConstraint, count int, excluded []storj.NodeID, alreadySelected []*SelectedNode) (_ []*SelectedNode, err error) {
	defer selectTask(&ctx)(&err)

	selector, found := s[p]
	if !found {
		return nil, Error.New("Placement is not defined: %d", p)
	}
	nodes, err := selector(ctx, requester, count, excluded, alreadySelected)
	if len(nodes) < count {
		return nodes, ErrNotEnoughNodes.New("requested from cache %d, found %d", count, len(nodes))
	}
	return nodes, err
}
