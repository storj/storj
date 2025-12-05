// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/location"
)

func TestState_SelectNonDistinct(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := joinNodes(
		createRandomNodes(2, "1.0.1", false, true),
		createRandomNodes(3, "1.0.2", false, true),
	)
	newNodes := joinNodes(
		createRandomNodes(2, "1.0.3", false, false),
		createRandomNodes(3, "1.0.4", false, false),
	)

	nodes := joinNodes(reputableNodes, newNodes)

	lastNet, err := nodeselection.CreateNodeAttribute("last_net")
	require.NoError(t, err)

	{ // select 5 non-distinct subnet reputable nodes
		state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
			0: {
				Selector: nodeselection.UnvettedSelector(0, nodeselection.AttributeGroupSelector(lastNet)),
			},
		})
		const selectCount = 5
		selected, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, selectCount)
	}

	{ // select 6 non-distinct subnet reputable and new nodes (50%)
		const selectCount = 6
		const newFraction = 0.5
		state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
			0: {
				Selector: nodeselection.UnvettedSelector(0.5, nodeselection.AttributeGroupSelector(lastNet)),
			},
		})
		selected, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, selectCount)
		require.Len(t, intersectLists(selected, reputableNodes), selectCount*(1-newFraction))
		require.Len(t, intersectLists(selected, newNodes), selectCount*newFraction)
	}

	{ // select 10 distinct subnet reputable and new nodes (100%), falling back to 5 reputable
		const selectCount = 10
		const newFraction = 1.0
		state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
			0: {
				Selector: nodeselection.UnvettedSelector(newFraction, nodeselection.AttributeGroupSelector(lastNet)),
			},
		})

		selected, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.NoError(t, err)

		require.Len(t, selected, selectCount)
		require.Len(t, intersectLists(selected, reputableNodes), 5)
		require.Len(t, intersectLists(selected, newNodes), 5)
	}
}

func TestState_UploadFilter(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	nodes := func() []*nodeselection.SelectedNode {
		nodes := joinNodes(
			createRandomNodes(3, "1.0.1", true, true),
			createRandomNodes(3, "1.0.2", true, true),
			createRandomNodes(3, "1.0.3", true, true),
		)
		for i := 0; i < 3; i++ {
			nodes[i].CountryCode = location.Germany
			nodes[i+3].CountryCode = location.Austria
			nodes[i+6].CountryCode = location.UnitedStates
		}
		return nodes
	}()

	deFilter, err := nodeselection.FilterFromString("country(\"DE\")", nil)
	require.NoError(t, err)

	euFilter, err := nodeselection.FilterFromString("country(\"EU\")", nil)
	require.NoError(t, err)

	state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
		0: {
			Selector:     nodeselection.RandomSelector(),
			NodeFilter:   euFilter,
			UploadFilter: nodeselection.NewExcludeFilter(deFilter),
		},
	})

	for i := 0; i < 10; i++ {
		selected, err := state.Select(ctx, storj.NodeID{}, 0, 3, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 3)
		for i := 0; i < len(selected); i++ {
			// Germany is excluded by upload filter, US is not included in the normal filter
			require.Equal(t, location.Austria, selected[i].CountryCode)
		}
	}

}

func TestState_SelectDistinct(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := joinNodes(
		createRandomNodes(2, "1.0.1", true, true),
		createRandomNodes(3, "1.0.2", true, true),
	)
	newNodes := joinNodes(
		createRandomNodes(2, "1.0.3", true, false),
		createRandomNodes(3, "1.0.4", true, false),
	)
	nodes := joinNodes(reputableNodes, newNodes)

	lastNet, err := nodeselection.CreateNodeAttribute("last_net")
	require.NoError(t, err)

	t.Run("select 2 distinct subnet reputable nodes", func(t *testing.T) {
		const selectCount = 2
		state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
			0: {
				Selector: nodeselection.UnvettedSelector(0, nodeselection.AttributeGroupSelector(lastNet)),
			},
		})

		selected, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.NoError(t, err)

		require.Len(t, selected, selectCount)
	})

	t.Run("try to select 5 distinct subnet reputable nodes, but there are only two 2 in the state", func(t *testing.T) {
		const selectCount = 5
		state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
			0: {
				Selector: nodeselection.UnvettedSelector(0, nodeselection.AttributeGroupSelector(lastNet)),
			},
		})

		selected, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.Error(t, err)
		require.Len(t, selected, 2)
	})

	t.Run("select 4 distinct subnet reputable and new nodes (50%)", func(t *testing.T) {
		const selectCount = 4
		const newFraction = 0.5
		state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
			0: {
				Selector: nodeselection.UnvettedSelector(newFraction, nodeselection.AttributeGroupSelector(lastNet)),
			},
		})

		selected, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, selectCount, nil)
		require.Len(t, intersectLists(selected, reputableNodes), selectCount*(1-newFraction))
		require.Len(t, intersectLists(selected, newNodes), selectCount*newFraction)
	})
}

func TestState_Select_Concurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := joinNodes(
		createRandomNodes(2, "1.0.1", false, true),
		createRandomNodes(3, "1.0.2", false, true),
	)
	newNodes := joinNodes(
		createRandomNodes(2, "1.0.3", false, false),
		createRandomNodes(3, "1.0.4", false, false),
	)

	nodes := joinNodes(reputableNodes, newNodes)

	state := nodeselection.InitState(ctx, nodes, map[storj.PlacementConstraint]nodeselection.Placement{
		0: {
			Selector: nodeselection.UnvettedSelector(0.5, nodeselection.RandomSelector()),
		},
	})

	var group errgroup.Group
	group.Go(func() error {
		const selectCount = 5
		nodes, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.Len(t, nodes, selectCount)
		return err
	})

	group.Go(func() error {
		const selectCount = 4
		nodes, err := state.Select(ctx, storj.NodeID{}, 0, selectCount, nil, nil)
		require.Len(t, nodes, selectCount)
		return err
	})
	require.NoError(t, group.Wait())
}

// createRandomNodes creates n random nodes all in the subnet.
func createRandomNodes(n int, subnet string, shareNets bool, vetted bool) []*nodeselection.SelectedNode {
	xs := make([]*nodeselection.SelectedNode, n)
	for i := range xs {
		addr := subnet + "." + strconv.Itoa(i) + ":8080"
		xs[i] = &nodeselection.SelectedNode{
			ID:         testrand.NodeID(),
			LastNet:    addr,
			LastIPPort: addr,
			Vetted:     vetted,
		}
		if shareNets {
			xs[i].LastNet = subnet
		} else {
			xs[i].LastNet = addr
		}
	}
	return xs
}

// joinNodes appends all slices into a single slice.
func joinNodes(lists ...[]*nodeselection.SelectedNode) []*nodeselection.SelectedNode {
	xs := []*nodeselection.SelectedNode{}
	for _, list := range lists {
		xs = append(xs, list...)
	}
	return xs
}

// intersectLists returns nodes that exist in both lists compared by ID.
func intersectLists(as, bs []*nodeselection.SelectedNode) []*nodeselection.SelectedNode {
	var xs []*nodeselection.SelectedNode

next:
	for _, a := range as {
		for _, b := range bs {
			if a.ID == b.ID {
				xs = append(xs, a)
				continue next
			}
		}
	}

	return xs
}
