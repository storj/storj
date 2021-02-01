// Copyright (C) 2020 Storj Labs, Incache.
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
)

func TestState_Select(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := joinNodes(
		createRandomNodes(2, "1.0.1"),
		createRandomNodes(3, "1.0.2"),
	)
	newNodes := joinNodes(
		createRandomNodes(2, "1.0.3"),
		createRandomNodes(3, "1.0.4"),
	)

	state := nodeselection.NewState(reputableNodes, newNodes)
	require.Equal(t, nodeselection.Stats{
		New:               5,
		Reputable:         5,
		NewDistinct:       2,
		ReputableDistinct: 2,
	}, state.Stats())

	{ // select 5 non-distinct subnet reputable nodes
		const selectCount = 5
		selected, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: 0,
			Distinct:    false,
			ExcludedIDs: nil,
		})
		require.NoError(t, err)
		require.Len(t, selected, selectCount)
	}

	{ // select 2 distinct subnet reputable nodes
		const selectCount = 2
		selected, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: 0,
			Distinct:    true,
			ExcludedIDs: nil,
		})
		require.NoError(t, err)
		require.Len(t, selected, selectCount)
	}

	{ // try to select 5 distinct subnet reputable nodes, but there are only two 2 in the state
		const selectCount = 5
		selected, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: 0,
			Distinct:    true,
			ExcludedIDs: nil,
		})
		require.Error(t, err)
		require.Len(t, selected, 2)
	}

	{ // select 6 non-distinct subnet reputable and new nodes (50%)
		const selectCount = 6
		const newFraction = 0.5
		selected, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: newFraction,
			Distinct:    false,
			ExcludedIDs: nil,
		})
		require.NoError(t, err)
		require.Len(t, selected, selectCount)
		require.Len(t, intersectLists(selected, reputableNodes), selectCount*(1-newFraction))
		require.Len(t, intersectLists(selected, newNodes), selectCount*newFraction)
	}

	{ // select 4 distinct subnet reputable and new nodes (50%)
		const selectCount = 4
		const newFraction = 0.5
		selected, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: newFraction,
			Distinct:    true,
			ExcludedIDs: nil,
		})
		require.NoError(t, err)
		require.Len(t, selected, selectCount)
		require.Len(t, intersectLists(selected, reputableNodes), selectCount*(1-newFraction))
		require.Len(t, intersectLists(selected, newNodes), selectCount*newFraction)
	}

	{ // select 10 distinct subnet reputable and new nodes (100%), falling back to 5 reputable
		const selectCount = 10
		const newFraction = 1.0
		selected, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: newFraction,
			Distinct:    false,
			ExcludedIDs: nil,
		})
		require.NoError(t, err)
		require.Len(t, selected, selectCount)
		require.Len(t, intersectLists(selected, reputableNodes), 5)
		require.Len(t, intersectLists(selected, newNodes), 5)
	}
}

func TestState_Select_Concurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	reputableNodes := joinNodes(
		createRandomNodes(2, "1.0.1"),
		createRandomNodes(3, "1.0.2"),
	)
	newNodes := joinNodes(
		createRandomNodes(2, "1.0.3"),
		createRandomNodes(3, "1.0.4"),
	)

	state := nodeselection.NewState(reputableNodes, newNodes)

	var group errgroup.Group
	group.Go(func() error {
		const selectCount = 5
		nodes, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: 0.5,
			Distinct:    false,
			ExcludedIDs: nil,
		})
		require.Len(t, nodes, selectCount)
		return err
	})

	group.Go(func() error {
		const selectCount = 4
		nodes, err := state.Select(ctx, nodeselection.Request{
			Count:       selectCount,
			NewFraction: 0.5,
			Distinct:    true,
			ExcludedIDs: nil,
		})
		require.Len(t, nodes, selectCount)
		return err
	})
	require.NoError(t, group.Wait())
}

// createRandomNodes creates n random nodes all in the subnet.
func createRandomNodes(n int, subnet string) []*nodeselection.Node {
	xs := make([]*nodeselection.Node, n)
	for i := range xs {
		addr := subnet + "." + strconv.Itoa(i) + ":8080"
		xs[i] = &nodeselection.Node{
			NodeURL: storj.NodeURL{
				ID:      testrand.NodeID(),
				Address: addr,
			},
			LastNet:    subnet,
			LastIPPort: addr,
		}
	}
	return xs
}

// joinNodes appends all slices into a single slice.
func joinNodes(lists ...[]*nodeselection.Node) []*nodeselection.Node {
	xs := []*nodeselection.Node{}
	for _, list := range lists {
		xs = append(xs, list...)
	}
	return xs
}

// intersectLists returns nodes that exist in both lists compared by ID.
func intersectLists(as, bs []*nodeselection.Node) []*nodeselection.Node {
	var xs []*nodeselection.Node

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
