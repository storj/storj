// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection_test

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/jtolio/mito"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/location"
)

func TestSelectByID(t *testing.T) {
	// create 3 nodes, 2 with same subnet
	// perform many node selections that selects 2 nodes
	// expect that the all node are selected ~33% of the time.
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// create 3 nodes, 2 with same subnet
	lastNetDuplicate := "1.0.1"
	subnetA1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}
	subnetA2 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	lastNetSingle := "1.0.2"
	subnetB1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*nodeselection.SelectedNode{subnetA1, subnetA2, subnetB1}
	selector := nodeselection.RandomSelector()(ctx, nodes, nil)

	const (
		reqCount       = 2
		executionCount = 10000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 2 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(ctx, storj.NodeID{}, reqCount, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, reqCount)
		for _, node := range selectedNodes {
			selectedNodeCount[node.ID]++
		}
	}

	subnetA1Count := float64(selectedNodeCount[subnetA1.ID])
	subnetA2Count := float64(selectedNodeCount[subnetA2.ID])
	subnetB1Count := float64(selectedNodeCount[subnetB1.ID])
	total := subnetA1Count + subnetA2Count + subnetB1Count
	assert.Equal(t, total, float64(reqCount*executionCount))

	const selectionEpsilon = 0.1
	const percent = 1.0 / 3.0
	assert.InDelta(t, subnetA1Count/total, percent, selectionEpsilon)
	assert.InDelta(t, subnetA2Count/total, percent, selectionEpsilon)
	assert.InDelta(t, subnetB1Count/total, percent, selectionEpsilon)
}

func TestSelectBySubnet(t *testing.T) {
	// create 3 nodes, 2 with same subnet
	// perform many node selections that selects 2 nodes
	// expect that the single node is selected 50% of the time
	// expect the 2 nodes with same subnet should each be selected 25% of time
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// create 3 nodes, 2 with same subnet
	lastNetDuplicate := "1.0.1"
	subnetA1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}
	subnetA2 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	lastNetSingle := "1.0.2"
	subnetB1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*nodeselection.SelectedNode{subnetA1, subnetA2, subnetB1}
	attribute, err := nodeselection.CreateNodeAttribute("last_net")
	require.NoError(t, err)
	selector := nodeselection.AttributeGroupSelector(attribute)(ctx, nodes, nil)

	const (
		reqCount       = 2
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 2 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(ctx, storj.NodeID{}, reqCount, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, reqCount)
		for _, node := range selectedNodes {
			selectedNodeCount[node.ID]++
		}
	}

	subnetA1Count := float64(selectedNodeCount[subnetA1.ID])
	subnetA2Count := float64(selectedNodeCount[subnetA2.ID])
	subnetB1Count := float64(selectedNodeCount[subnetB1.ID])
	total := subnetA1Count + subnetA2Count + subnetB1Count
	assert.Equal(t, total, float64(reqCount*executionCount))

	// expect that the single node is selected 50% of the time
	// expect the 2 nodes with same subnet should each be selected 25% of time
	nodeID1total := subnetA1Count / total
	nodeID2total := subnetA2Count / total

	const (
		selectionEpsilon = 0.1
		uniqueSubnet     = 0.5
	)

	// we expect that the 2 nodes from the same subnet should be
	// selected roughly the same percent of the time
	assert.InDelta(t, nodeID2total, nodeID1total, selectionEpsilon)

	// the node from the unique subnet should be selected exactly half of the time
	nodeID3total := subnetB1Count / total
	assert.Equal(t, nodeID3total, uniqueSubnet)
}

func TestSelectBySubnetOneAtATime(t *testing.T) {
	// create 3 nodes, 2 with same subnet
	// perform many node selections that selects 1 node
	// expect that the single node is selected 50% of the time
	// expect the 2 nodes with same subnet should each be selected 25% of time
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// create 3 nodes, 2 with same subnet
	lastNetDuplicate := "1.0.1"
	subnetA1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}
	subnetA2 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	lastNetSingle := "1.0.2"
	subnetB1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*nodeselection.SelectedNode{subnetA1, subnetA2, subnetB1}
	attribute, err := nodeselection.CreateNodeAttribute("last_net")
	require.NoError(t, err)
	selector := nodeselection.AttributeGroupSelector(attribute)(ctx, nodes, nil)

	const (
		reqCount       = 1
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 1 node
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(ctx, storj.NodeID{}, reqCount, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, reqCount)
		for _, node := range selectedNodes {
			selectedNodeCount[node.ID]++
		}
	}

	subnetA1Count := float64(selectedNodeCount[subnetA1.ID])
	subnetA2Count := float64(selectedNodeCount[subnetA2.ID])
	subnetB1Count := float64(selectedNodeCount[subnetB1.ID])
	total := subnetA1Count + subnetA2Count + subnetB1Count
	assert.Equal(t, total, float64(reqCount*executionCount))

	const (
		selectionEpsilon = 0.1
		uniqueSubnet     = 0.5
	)

	// we expect that the 2 nodes from the same subnet should be
	// selected roughly the same ~25% percent of the time
	assert.InDelta(t, subnetA2Count/total, subnetA1Count/total, selectionEpsilon)

	// expect that the single node is selected ~50% of the time
	assert.InDelta(t, subnetB1Count/total, uniqueSubnet, selectionEpsilon)
}

func TestSelectFiltered(t *testing.T) {

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// create 3 nodes, 2 with same subnet
	lastNetDuplicate := "1.0.1"
	firstID := testrand.NodeID()
	subnetA1 := &nodeselection.SelectedNode{
		ID:         firstID,
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}

	secondID := testrand.NodeID()
	subnetA2 := &nodeselection.SelectedNode{
		ID:         secondID,
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	thirdID := testrand.NodeID()
	lastNetSingle := "1.0.2"
	subnetB1 := &nodeselection.SelectedNode{
		ID:         thirdID,
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*nodeselection.SelectedNode{subnetA1, subnetA2, subnetB1}

	selector := nodeselection.RandomSelector()(ctx, nodes, nil)
	selected, err := selector(ctx, storj.NodeID{}, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, selected, 3)
	selected, err = selector(ctx, storj.NodeID{}, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, selected, 3)

	selector = nodeselection.RandomSelector()(ctx, nodes, nodeselection.NodeFilters{}.WithExcludedIDs([]storj.NodeID{firstID, secondID}))
	selected, err = selector(ctx, storj.NodeID{}, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, selected, 1)
}

func TestSelectFilteredMulti(t *testing.T) {
	// four subnets with 3 nodes in each. Only one per subnet is located in Germany.
	// Algorithm should pick the German one from each subnet, and 4 nodes should be possible to be picked.

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var nodes []*nodeselection.SelectedNode

	for i := 0; i < 12; i++ {
		nodes = append(nodes, &nodeselection.SelectedNode{
			ID:          testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
			LastNet:     fmt.Sprintf("68.0.%d", i/3),
			LastIPPort:  fmt.Sprintf("68.0.%d.%d:1000", i/3, i),
			CountryCode: location.Germany + location.CountryCode(i%3),
		})

	}

	filter := nodeselection.NodeFilters{}.WithCountryFilter(location.NewSet(location.Germany))
	attribute, err := nodeselection.CreateNodeAttribute("last_net")
	require.NoError(t, err)
	selector := nodeselection.AttributeGroupSelector(attribute)(ctx, nodes, filter)
	for i := 0; i < 100; i++ {
		selected, err := selector(ctx, storj.NodeID{}, 4, nil, nil)
		require.NoError(t, err)
		assert.Len(t, selected, 4)
	}
}

func TestFilterSelector(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	list := nodeselection.AllowedNodesFilter([]storj.NodeID{
		testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID,
		testidentity.MustPregeneratedIdentity(2, storj.LatestIDVersion()).ID,
		testidentity.MustPregeneratedIdentity(3, storj.LatestIDVersion()).ID,
	})

	selector := nodeselection.FilterSelector(nodeselection.NewExcludeFilter(list), nodeselection.RandomSelector())

	// initialize the node space
	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 10; i++ {
		nodes = append(nodes, &nodeselection.SelectedNode{
			ID: testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
		})
	}

	initialized := selector(ctx, nodes, nil)
	for i := 0; i < 100; i++ {
		selected, err := initialized(ctx, storj.NodeID{}, 3, []storj.NodeID{}, nil)
		require.NoError(t, err)
		for _, s := range selected {
			for _, w := range list {
				// make sure we didn't choose from the white list, as they are excluded
				require.NotEqual(t, s.ID, w)
			}
		}
	}
}

func TestBalancedSelector(t *testing.T) {
	attribute, err := nodeselection.CreateNodeAttribute("tag:owner")
	require.NoError(t, err)

	ownerCounts := map[string]int{"A": 3, "B": 30, "C": 30, "D": 5}
	var nodes []*nodeselection.SelectedNode

	idIndex := 0
	for owner, count := range ownerCounts {
		for i := 0; i < count; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testidentity.MustPregeneratedIdentity(idIndex, storj.LatestIDVersion()).ID,
				Tags: nodeselection.NodeTags{
					{
						Name:  "owner",
						Value: []byte(owner),
					},
				},
			})
			idIndex++
		}
	}

	ctx := testcontext.New(t)
	selector := nodeselection.BalancedGroupBasedSelector(attribute, nil)(ctx, nodes, nil)

	var badSelection atomic.Int64
	for i := 0; i < 1000; i++ {
		ctx.Go(func() error {
			selectedNodes, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
			if err != nil {
				t.Log("Selection is failed", err.Error())
				badSelection.Add(1)
				return nil
			}

			if len(selectedNodes) != 10 {
				t.Log("Wrong number of nodes are selected", len(selectedNodes))
				badSelection.Add(1)
				return nil

			}

			histogram := map[string]int{}
			for _, node := range selectedNodes {
				histogram[attribute(*node)] = histogram[attribute(*node)] + 1
			}
			for _, c := range histogram {
				if c > 5 {
					badSelection.Add(1)
					break
				}
			}
			return nil
		})
	}
	ctx.Wait()
	// there is a very-very low chance to have wrong selection if we select one from A
	// and all the other random selection will select the same node again
	require.Equal(t, int64(0), badSelection.Load())
}

func TestBalancedSelectorWithExisting(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	attribute, err := nodeselection.CreateNodeAttribute("tag:owner")
	require.NoError(t, err)

	ownerCounts := map[string]int{"A": 3, "B": 10, "C": 30, "D": 5, "E": 1}
	var nodes []*nodeselection.SelectedNode

	var excluded []storj.NodeID
	var alreadySelected []*nodeselection.SelectedNode

	idIndex := 0
	for owner, count := range ownerCounts {
		for i := 0; i < count; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testidentity.MustPregeneratedIdentity(idIndex, storj.LatestIDVersion()).ID,
				Tags: nodeselection.NodeTags{
					{
						Name:  "owner",
						Value: []byte(owner),
					},
				},
			})
			idIndex++
			if owner == "A" {
				excluded = append(excluded, nodes[len(nodes)-1].ID)
			}
			if owner == "B" && len(alreadySelected) < 9 {
				alreadySelected = append(alreadySelected, nodes[len(nodes)-1])
			}
		}
	}

	selector := nodeselection.BalancedGroupBasedSelector(attribute, nil)(ctx, nodes, nil)

	histogram := map[string]int{}
	for i := 0; i < 1000; i++ {
		selectedNodes, err := selector(ctx, storj.NodeID{}, 7, excluded, alreadySelected)
		require.NoError(t, err)

		require.Len(t, selectedNodes, 7)

		for _, node := range selectedNodes {
			histogram[attribute(*node)]++
		}
	}
	// from the initial {"A": 3, "B": 10, "C": 30, "D": 5, "E": 1}

	// A is fully excluded
	require.Equal(t, 0, histogram["A"])

	// 9 out of 10 are excluded, we always select the remaining one
	require.Equal(t, 1000, histogram["B"])

	require.Greater(t, histogram["C"], 1000)
	require.Greater(t, histogram["D"], 1000)

	// one option, we always select one, as we choose 7 from 4 groups
	require.Equal(t, 1000, histogram["E"])

}

func TestUnvettedSelector(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 20; i++ {
		node := &nodeselection.SelectedNode{
			ID: testrand.NodeID(),
		}
		if i < 10 {
			node.Vetted = true
		}

		nodes = append(nodes, node)
	}

	t.Run("0 new nodes", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(1.0, nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes[:10], nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 10)
			require.Equal(t, 0, countUnvetted(selected))
		}
	})

	t.Run("25% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0.25, nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 5)
			require.Equal(t, 1, countUnvetted(selected))
		}
	})

	t.Run("15% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0.15, nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			// The faction result in less than 1 node, so it randonly decide if 0 or 1 vetted node is
			// selected.
			require.InDelta(t, 0, countUnvetted(selected), 1)
		}
	})

	t.Run("0.01% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0.0001, nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			// The faction result in less than 1 node, so it randomly decide if 0 or 1 vetted node is
			// selected.
			require.InDelta(t, 0, countUnvetted(selected), 1)
		}
	})

	t.Run("0% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0, nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Zero(t, countUnvetted(selected))
		}
	})

	t.Run("negative % of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(-1, nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Zero(t, countUnvetted(selected))
		}
	})

	t.Run("NaN % of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(math.NaN(), nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Zero(t, countUnvetted(selected))
		}
	})
}

func TestUnvettedSelectorFraction(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 100; i++ {
		node := &nodeselection.SelectedNode{
			ID: testrand.NodeID(),
		}
		if i >= 5 {
			node.Vetted = true
		}

		nodes = append(nodes, node)
	}

	// now we have 5% vetted nodes. When we define 10% vetted fraction,it should be used as upper limit, but 5% should be used instead of overusage.

	selectorInit := nodeselection.UnvettedSelector(0.1, nodeselection.RandomSelector())
	selector := selectorInit(ctx, nodes, nil)

	for i := 0; i < 100; i++ {
		selected, err := selector(ctx, storj.NodeID{}, 50, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 50)
		require.Equal(t, 2, countUnvetted(selected))
	}

}
func TestChoiceOfTwo(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	tracker := &mockTracker{
		trustedUplink: testrand.NodeID(),
	}

	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 20; i++ {
		node := &nodeselection.SelectedNode{
			ID: testrand.NodeID(),
		}
		if i < 10 {
			node.Email = "slow"
			tracker.slowNodes = append(tracker.slowNodes, node.ID)
		}
		nodes = append(nodes, node)
	}

	selector := nodeselection.ChoiceOfTwo(nodeselection.Compare(tracker), nodeselection.RandomSelector())
	initializedSelector := selector(ctx, nodes, nil)

	for i := 0; i < 100; i++ {
		selectedNodes, err := initializedSelector(ctx, tracker.trustedUplink, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, 10)
		slowNodes := countSlowNodes(selectedNodes)
		// we have 10 slow nodes, and 10 fast
		// if all the slow nodes are pair-selected: we will have 5 slow and 5 fast in the selection
		// we can be more lucky, when slow nodes got fast pairs
		require.Less(t, slowNodes, 6)
	}

	suboptimal := 0
	for i := 0; i < 1000; i++ {
		selectedNodes, err := initializedSelector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, 10)

		slowCount := countSlowNodes(selectedNodes)

		// we don't filter out slow nodes, as the requester is not the trusted nodeID
		if slowCount >= 6 {
			suboptimal++
		}
	}

	// don't know the math, but usually it's ~320
	require.Greater(t, suboptimal, 50)
}

func TestChoiceOfN(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	tracker := &mockTracker{
		trustedUplink: testrand.NodeID(),
	}

	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 30; i++ {
		node := &nodeselection.SelectedNode{
			ID: testrand.NodeID(),
		}
		if i < 20 {
			node.Email = "slow"
			tracker.slowNodes = append(tracker.slowNodes, node.ID)
		}
		nodes = append(nodes, node)
	}

	selector := nodeselection.ChoiceOfN(nodeselection.Compare(tracker), 3, nodeselection.RandomSelector())
	initializedSelector := selector(ctx, nodes, nil)

	for i := 0; i < 100; i++ {
		selectedNodes, err := initializedSelector(ctx, tracker.trustedUplink, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, 10)
		slowNodes := countSlowNodes(selectedNodes)
		// we have 20 slow nodes, and 10 fast
		// if all the slow nodes are triple-selected: we will have 6 slow and 4 fast
		// we can be more lucky, when slow nodes got fast pairs
		require.Less(t, slowNodes, 7)
	}

	suboptimal := 0
	for i := 0; i < 1000; i++ {
		selectedNodes, err := initializedSelector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, 10)

		slowCount := countSlowNodes(selectedNodes)

		// we don't filter out slow nodes, as the requester is not the trusted nodeID
		if slowCount >= 7 {
			suboptimal++
		}
	}

	// don't know the math, but usually it's ~560
	require.Greater(t, suboptimal, 87)
}

func TestFilterBest(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	tracker := &mockTracker{
		trustedUplink: storj.NodeID{},
	}

	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 20; i++ {
		node := &nodeselection.SelectedNode{
			ID: testrand.NodeID(),
		}
		if i < 10 {
			node.Email = "slow"
			tracker.slowNodes = append(tracker.slowNodes, node.ID)
		}
		nodes = append(nodes, node)
	}

	t.Run("keep best 40%", func(t *testing.T) {
		selectorInit := nodeselection.FilterBest(tracker, "40%", "", nodeselection.RandomSelector())
		for i := 0; i < 2; i++ {
			nodeSelector := selectorInit(ctx, nodes, nil)
			for i := 0; i < 100; i++ {
				selected, err := nodeSelector(ctx, storj.NodeID{}, 8, nil, nil)
				require.NoError(t, err)
				require.Len(t, selected, 8)
				require.Equal(t, 0, countSlowNodes(selected))
			}
		}
	})

	t.Run("keep best 8", func(t *testing.T) {
		selectorInit := nodeselection.FilterBest(tracker, "8", "", nodeselection.RandomSelector())
		nodeSelector := selectorInit(ctx, nodes, nil)
		for i := 0; i < 10; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 2, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 2)
			require.Equal(t, 0, countSlowNodes(selected))
		}
	})

	t.Run("cut off worst 30", func(t *testing.T) {
		selectorInit := nodeselection.FilterBest(tracker, "-30", "", nodeselection.RandomSelector())
		nodeSelector := selectorInit(ctx, nodes, nil)
		for i := 0; i < 10; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 0)
			require.Equal(t, 0, countSlowNodes(selected))
		}
	})
}

func TestFilterBestOfN(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	tracker := &mockTracker{
		trustedUplink: storj.NodeID{},
	}

	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 20; i++ {
		node := &nodeselection.SelectedNode{
			ID: testrand.NodeID(),
		}
		if i < 10 {
			node.Email = "slow"
			tracker.slowNodes = append(tracker.slowNodes, node.ID)
		}
		nodes = append(nodes, node)
	}

	t.Run("fastest 10 out of 20", func(t *testing.T) {
		selectorInit := nodeselection.BestOfN(tracker, 2.0, nodeselection.RandomSelector())
		nodeSelector := selectorInit(ctx, nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 10)
			require.Equal(t, 0, countSlowNodes(selected))
		}
	})

	t.Run("fastest 10 out of 5", func(t *testing.T) {
		selectorInit := nodeselection.BestOfN(tracker, 0.5, nodeselection.RandomSelector())
		nodeSelector := selectorInit(ctx, nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 5)
		}
	})
}

func TestEqSelector(t *testing.T) {
	surgeTag, err := nodeselection.CreateNodeAttribute("tag:surge")
	require.NoError(t, err)
	selected := nodeselection.EqualSelector(surgeTag, "true")

	require.True(t, selected(nodeselection.SelectedNode{
		ID: testrand.NodeID(),
		Tags: nodeselection.NodeTags{
			{
				Name:  "surge",
				Value: []byte("true"),
			},
		},
	}))

	require.False(t, selected(nodeselection.SelectedNode{
		ID: testrand.NodeID(),
		Tags: nodeselection.NodeTags{
			{
				Name:  "surge",
				Value: []byte("false"),
			},
		},
	}))
}

func TestIfSelector(t *testing.T) {
	lastNetAttibute, err := nodeselection.CreateNodeAttribute("last_net")
	require.NoError(t, err)
	lastIpPortAttribute, err := nodeselection.CreateNodeAttribute("last_ip_port")
	require.NoError(t, err)

	selectedTrue := nodeselection.IfSelector(
		func(node nodeselection.SelectedNode) bool { return true }, lastNetAttibute, lastIpPortAttribute)
	selectedFalse := nodeselection.IfSelector(
		func(node nodeselection.SelectedNode) bool { return false }, lastNetAttibute, lastIpPortAttribute)

	selectedNode := nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    "1.0.1",
		LastIPPort: "1.0.1.5:8080",
	}

	require.Equal(t, lastNetAttibute(selectedNode), selectedTrue(selectedNode))
	require.Equal(t, lastIpPortAttribute(selectedNode), selectedFalse(selectedNode))
}

func TestIfWithEqSelector(t *testing.T) {
	// create 4 nodes, 2 per subnet
	// perform many node selections that selects 1 node
	// use if selector such that one set of nodes use last_ip_port and the other use last_net
	// expect that the nodes selected based on last_ip_port are selected as often as the sum
	// of the other two nodes sharing a subnet
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// create 3 nodes, 2 with same subnet
	lastNetA := "1.0.1"
	subnetA1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetA,
		LastIPPort: lastNetA + ".4:8080",
		Tags: nodeselection.NodeTags{
			{
				Name:  "owner",
				Value: []byte("public"),
			},
		},
	}
	subnetA2 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetA,
		LastIPPort: lastNetA + ".5:8080",
		Tags: nodeselection.NodeTags{
			{
				Name:  "owner",
				Value: []byte("public"),
			},
		},
	}

	lastNetB := "1.0.2"
	subnetB1 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetB,
		LastIPPort: lastNetB + ".4:8080",
		Tags: nodeselection.NodeTags{
			{
				Name:  "owner",
				Value: []byte("storj"),
			},
			{
				Name:  "surge",
				Value: []byte("true"),
			},
		},
	}
	subnetB2 := &nodeselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetB,
		LastIPPort: lastNetB + ".5:8080",
		Tags: nodeselection.NodeTags{
			{
				Name:  "owner",
				Value: []byte("storj"),
			},
			{
				Name:  "surge",
				Value: []byte("true"),
			},
		},
	}

	nodes := []*nodeselection.SelectedNode{subnetA1, subnetA2, subnetB1, subnetB2}

	surgeTag, err := nodeselection.CreateNodeAttribute("tag:surge")
	require.NoError(t, err)
	lastNetAttribute, err := nodeselection.CreateNodeAttribute("last_net")
	require.NoError(t, err)
	lastIpPortAttribute, err := nodeselection.CreateNodeAttribute("last_ip_port")
	require.NoError(t, err)

	selector := nodeselection.BalancedGroupBasedSelector(nodeselection.IfSelector(
		nodeselection.EqualSelector(surgeTag, "true"), lastIpPortAttribute, lastNetAttribute), nil)(ctx, nodes, nil)

	const (
		reqCount       = 3
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 3 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(ctx, storj.NodeID{}, reqCount, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, reqCount)
		for _, node := range selectedNodes {
			selectedNodeCount[node.ID]++
		}
	}

	subnetA1Count := float64(selectedNodeCount[subnetA1.ID])
	subnetA2Count := float64(selectedNodeCount[subnetA2.ID])
	subnetB1Count := float64(selectedNodeCount[subnetB1.ID])
	subnetB2Count := float64(selectedNodeCount[subnetB1.ID])
	total := subnetA1Count + subnetA2Count + subnetB1Count + subnetB2Count
	assert.Equal(t, total, float64(reqCount*executionCount))

	nodeID1total := subnetA1Count / total
	nodeID2total := subnetA2Count / total
	nodeID3total := subnetB1Count / total
	nodeID4total := subnetB2Count / total

	const selectionEpsilon = 0.05

	// we expect that 2 nodes from the same subnet should be
	// selected roughly the same percent of the time
	assert.InDelta(t, nodeID1total, nodeID2total, selectionEpsilon)
	assert.InDelta(t, nodeID3total, nodeID4total, selectionEpsilon)

	// when their totals are combined, the 2 nodes in the subnet with "public" owner
	// should be selected about as often as one of the nodes in the subnet with "storj" owner
	assert.InDelta(t, nodeID1total+nodeID2total, nodeID3total, selectionEpsilon)
}

func TestDualSelector(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	slowFilter, err := nodeselection.NewAttributeFilter("email", "==", "slow")
	require.NoError(t, err)
	fastFilter, err := nodeselection.NewAttributeFilter("email", "==", "fast")
	require.NoError(t, err)

	t.Run("3 from slow, 7 from remaining", func(t *testing.T) {
		nodes, _ := generateNodes(10, 10)

		selectorInit := nodeselection.DualSelector(
			0.3,
			nodeselection.FilteredSelector(slowFilter, nodeselection.RandomSelector()),
			nodeselection.FilteredSelector(fastFilter, nodeselection.RandomSelector()),
		)
		nodeSelector := selectorInit(ctx, nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 10)
			require.Equal(t, 3, countSlowNodes(selected))
		}
	})

	t.Run("3 from slow, 7 from remaining, fast only", func(t *testing.T) {
		nodes, _ := generateNodes(0, 20)

		selectorInit := nodeselection.DualSelector(
			0.3,
			nodeselection.FilteredSelector(slowFilter, nodeselection.RandomSelector()),
			nodeselection.FilteredSelector(fastFilter, nodeselection.RandomSelector()),
		)
		nodeSelector := selectorInit(ctx, nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 10)
			require.Equal(t, 0, countSlowNodes(selected))
		}
	})

	t.Run("3 from slow, 7 from fast, slow only", func(t *testing.T) {
		nodes, _ := generateNodes(20, 0)

		selectorInit := nodeselection.DualSelector(
			0.3,
			nodeselection.FilteredSelector(slowFilter, nodeselection.RandomSelector()),
			nodeselection.FilteredSelector(fastFilter, nodeselection.RandomSelector()),
		)
		nodeSelector := selectorInit(ctx, nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 3)
			require.Equal(t, 3, countSlowNodes(selected))
		}
	})

	t.Run("using fraction", func(t *testing.T) {
		nodes, _ := generateNodes(10, 10)

		selectorInit := nodeselection.DualSelector(
			0.25,
			nodeselection.FilteredSelector(slowFilter, nodeselection.RandomSelector()),
			nodeselection.FilteredSelector(fastFilter, nodeselection.RandomSelector()),
		)
		nodeSelector := selectorInit(ctx, nodes, nil)
		slowCounts := 0
		allCounts := 0
		for i := 0; i < 1000; i++ {
			selected, err := nodeSelector(ctx, storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			slowNodeCount := countSlowNodes(selected)
			slowCounts += slowNodeCount
			allCounts += len(selected)
			require.Len(t, selected, 10)
			require.Contains(t, []int{2, 3}, slowNodeCount)
		}

		// this should be very close to 2.5
		ratio := float64(slowCounts) / float64(allCounts)
		require.InDelta(t, 0.25, ratio, 0.05)

	})
}

func generateNodes(slow int, fast int) ([]*nodeselection.SelectedNode, *mockTracker) {
	tracker := &mockTracker{
		trustedUplink: storj.NodeID{},
	}
	var nodes []*nodeselection.SelectedNode
	for i := 0; i < slow+fast; i++ {
		node := &nodeselection.SelectedNode{
			ID: testrand.NodeID(),
		}
		if i < slow {
			node.Email = "slow"
			tracker.slowNodes = append(tracker.slowNodes, node.ID)
		} else {
			node.Email = "fast"
		}
		nodes = append(nodes, node)
	}
	return nodes, tracker
}

// mockSelector returns only 1 success, for slow nodes, but only if trustedUplink does ask it.
type mockTracker struct {
	trustedUplink storj.NodeID
	slowNodes     []storj.NodeID
}

var _ nodeselection.UploadSuccessTracker = (*mockTracker)(nil)

func (m *mockTracker) Get(uplink storj.NodeID) func(node *nodeselection.SelectedNode) float64 {
	return func(node *nodeselection.SelectedNode) float64 {
		if uplink == m.trustedUplink {
			for _, slow := range m.slowNodes {
				if slow == node.ID {
					return 1
				}
			}
		}
		return 10
	}
}

func countSlowNodes(nodes []*nodeselection.SelectedNode) int {
	slowCount := 0
	for _, node := range nodes {
		if node.Email == "slow" {
			slowCount++
		}
	}
	return slowCount
}

func countUnvetted(nodes []*nodeselection.SelectedNode) int {
	unvetted := 0
	for _, node := range nodes {
		if !node.Vetted {
			unvetted++
		}
	}

	return unvetted
}

func TestRoundWithProbability(t *testing.T) {
	for _, n := range []float64{0, 0.1, 0.5, 0.9, 1, 0.999, 12.8} {
		t.Run(fmt.Sprintf("%f", n), func(t *testing.T) {
			count := 10000
			sum := 0
			ceil := int(math.Ceil(n))
			floor := int(math.Floor(n))
			for i := 0; i < count; i++ {
				rounded := nodeselection.RoundWithProbability(n)
				require.Contains(t, []int{ceil, floor}, rounded)
				sum += rounded
			}
			require.InDelta(t, n, float64(sum)/float64(count), 0.1)
		})
	}
}

func TestMaxGroup(t *testing.T) {
	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 10; i++ {
		nodes = append(nodes, &nodeselection.SelectedNode{
			ID:         testrand.NodeID(),
			LastIPPort: fmt.Sprintf("1.0.0.%d:8080", i),
		})
	}
	attribute, err := nodeselection.CreateNodeAttribute("last_ip")
	require.NoError(t, err)
	require.Equal(t, float64(1), nodeselection.MaxGroup(attribute)(storj.NodeID{}, nodes))
	nodes[9].LastIPPort = "1.0.0.1:8081"
	nodes[0].LastIPPort = "1.0.0.1:8082"
	nodes[5].LastIPPort = "1.0.0.1:8083"
	require.Equal(t, float64(4), nodeselection.MaxGroup(attribute)(storj.NodeID{}, nodes))
}

func TestPieceCount(t *testing.T) {
	require.Equal(t, float64(100), nodeselection.PieceCount(10).Get(storj.NodeID{})(&nodeselection.SelectedNode{
		PieceCount: 1000,
	}))
}

func TestLastBut(t *testing.T) {
	var nodes []*nodeselection.SelectedNode
	for i := 0; i < 10; i++ {
		node := &nodeselection.SelectedNode{
			ID:         testrand.NodeID(),
			LastIPPort: fmt.Sprintf("1.0.0.%d:8080", i),
		}
		node.PieceCount = int64(i * 10)
		nodes = append(nodes, node)
	}

	require.Equal(t, float64(0), nodeselection.LastBut(nodeselection.PieceCount(10), 0)(storj.NodeID{}, nodes))
	require.Equal(t, float64(1), nodeselection.LastBut(nodeselection.PieceCount(10), 1)(storj.NodeID{}, nodes))
	require.True(t, math.IsNaN(nodeselection.LastBut(nodeselection.PieceCount(10), 100)(storj.NodeID{}, nodes)))
	require.Equal(t, float64(60), nodeselection.LastBut(nodeselection.ScoreNodeFunc(func(uplink storj.NodeID, node *nodeselection.SelectedNode) float64 {
		if node.PieceCount < 50 {
			return math.NaN()
		}
		return float64(node.PieceCount)
	}), 1)(storj.NodeID{}, nodes))

}

func TestChoiceOfNSelection(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// pre-generate 4 selections
	var selections [][]*nodeselection.SelectedNode
	for i := 0; i < 4; i++ {
		var selection []*nodeselection.SelectedNode
		for j := 0; j < 10; j++ {
			if i == 3 && j > 5 {
				break
			}
			selection = append(selection, &nodeselection.SelectedNode{
				ID:         testrand.NodeID(),
				Email:      fmt.Sprintf("%d@%d", i, j),
				PieceCount: int64(i*1000 + j*100),
			})
		}
		selections = append(selections, selection)
	}

	ix := -1
	predictableSelector := func(ctx context.Context, nodes []*nodeselection.SelectedNode, filter nodeselection.NodeFilter) nodeselection.NodeSelector {
		return func(ctx context.Context, requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			ix++
			return selections[ix], nil
		}
	}
	selector := nodeselection.ChoiceOfNSelection(3, predictableSelector, nodeselection.LastBut(nodeselection.Desc(nodeselection.PieceCount(10)), 0))
	initializedSelector := selector(ctx, nil, nil)
	selection, err := initializedSelector(ctx, storj.NodeID{}, 10, nil, nil)
	require.NoError(t, err)

	require.Len(t, selection, 10)

	// First group has the less piece counts
	require.Equal(t, "0@", selection[0].Email[0:2])
}

func TestMin(t *testing.T) {
	tracker := &mockTracker{}

	// score of node1 is 10 (default)
	node1 := testrand.NodeID()

	// score of node2 is 1 (slow node)
	node2 := testrand.NodeID()
	tracker.slowNodes = append(tracker.slowNodes, node2)

	env := map[interface{}]interface{}{
		"tracker": tracker,
	}
	nodeselection.AddArithmetic(env)
	test := func(expression string, node storj.NodeID, expected float64) {
		evaluated, err := mito.Eval(expression, env)
		require.NoError(t, err)
		f := evaluated.(nodeselection.ScoreNode).Get(storj.NodeID{})(&nodeselection.SelectedNode{
			ID: node,
		})
		require.Equal(t, expected, f)
	}

	test("min(tracker,1)", node1, float64(1))
	test("min(tracker,1)", node2, float64(1))
	test("min(tracker,1.2)", node2, float64(1))

	test("min(15.9,tracker)", node1, float64(10))
	test("min(15,tracker)", node1, float64(10))
	test("min(15,tracker)", node2, float64(1))
}

func TestWeightedSelector(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var nodes []*nodeselection.SelectedNode
	idIndex := 0

	for i := 0; i < 100; i++ {
		nodes = append(nodes, &nodeselection.SelectedNode{
			ID:    testidentity.MustPregeneratedIdentity(idIndex, storj.LatestIDVersion()).ID,
			Email: fmt.Sprintf("%d@%d", i, i),
			Tags: nodeselection.NodeTags{
				{
					Name:   "weight",
					Signer: storj.NodeID{},
					Value:  []byte(strconv.Itoa(100)),
				},
			},
		})
		idIndex++
	}

	// 3x more chance to be selected --> selecting 10 nodes --> very high chance, for being selected (at least once)
	nodes[0].Tags[0].Value = []byte("500")
	val, err := nodeselection.CreateNodeValue("tag:1111111111111111111111111111111112m1s9K/weight?100")
	require.NoError(t, err)

	selector := nodeselection.WeightedSelector(val, nil)(ctx, nodes, nil)

	histogram := map[storj.NodeID]int{}

	for i := 0; i < 10000; i++ {
		selectedNodes, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, 10)

		for _, node := range selectedNodes {
			histogram[node.ID]++
		}
	}

	selector = nodeselection.WeightedSelector(val, nodeselection.NodeFilterFunc(func(node *nodeselection.SelectedNode) bool {
		return false
	}))(ctx, nodes, nil)
	selectedNodes, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, selectedNodes, 0)

	// specific node selected at least 3 times more
	require.Greater(t, float64(histogram[nodes[0].ID])/float64(histogram[nodes[1].ID]), float64(3))

}

func TestReduce(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("no constraints", func(t *testing.T) {
		nodes := []*nodeselection.SelectedNode{
			{ID: testrand.NodeID(), LastNet: "192.168.1.0/24"},
			{ID: testrand.NodeID(), LastNet: "192.168.2.0/24"},
		}

		selectorInit := nodeselection.Reduce(nodeselection.RandomSelector(), nil)
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 2, nil, nil)
		require.NoError(t, err)
		require.Greater(t, len(selected), 0)
	})

	t.Run("single constraint - functional test", func(t *testing.T) {
		// Create nodes with different subnets to test AtLeast constraint
		nodes := []*nodeselection.SelectedNode{
			{ID: testrand.NodeID(), LastNet: "192.168.1.0/24"},
			{ID: testrand.NodeID(), LastNet: "192.168.2.0/24"},
			{ID: testrand.NodeID(), LastNet: "192.168.3.0/24"},
			{ID: testrand.NodeID(), LastNet: "192.168.4.0/24"},
			{ID: testrand.NodeID(), LastNet: "192.168.5.0/24"},
		}

		attr, err := nodeselection.CreateNodeAttribute("last_net")
		require.NoError(t, err)

		selectorInit := nodeselection.Reduce(
			nodeselection.RandomSelector(),
			nil,
			nodeselection.AtLeast(attr, 2), // Include nodes until we have 2 different groups
		)
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 2, "Should include exactly 2 nodes (when we have 2 different groups, needMore becomes false)")
	})

	t.Run("multiple constraints", func(t *testing.T) {
		// Create nodes with different attributes
		nodes := []*nodeselection.SelectedNode{
			{ID: testrand.NodeID(), LastNet: "192.168.1.0/24", CountryCode: location.Germany},
			{ID: testrand.NodeID(), LastNet: "192.168.1.0/24", CountryCode: location.Germany},
			{ID: testrand.NodeID(), LastNet: "192.168.2.0/24", CountryCode: location.Austria},
			{ID: testrand.NodeID(), LastNet: "192.168.2.0/24", CountryCode: location.Austria},
		}

		subnetAttr, err := nodeselection.CreateNodeAttribute("last_net")
		require.NoError(t, err)
		countryAttr, err := nodeselection.CreateNodeAttribute("country")
		require.NoError(t, err)

		// Two constraints: need at most 1 per subnet AND at most 1 per country
		selectorInit := nodeselection.Reduce(
			nodeselection.RandomSelector(),
			nil,
			nodeselection.AtLeast(subnetAttr, 1),  // Include while subnet count <= 1
			nodeselection.AtLeast(countryAttr, 1), // Include while country count <= 1
		)
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 4, nil, nil)
		require.NoError(t, err)

		require.Len(t, selected, 1)
	})

	t.Run("with node filter", func(t *testing.T) {
		nodes := []*nodeselection.SelectedNode{
			{ID: testrand.NodeID(), LastNet: "192.168.1.0/24", CountryCode: location.Germany},
			{ID: testrand.NodeID(), LastNet: "192.168.2.0/24", CountryCode: location.Austria},
			{ID: testrand.NodeID(), LastNet: "192.168.3.0/24", CountryCode: location.Germany},
		}

		// Filter that only allows German nodes
		filter := nodeselection.NodeFilterFunc(func(node *nodeselection.SelectedNode) bool {
			return node.CountryCode == location.Germany
		})

		selectorInit := nodeselection.Reduce(nodeselection.RandomSelector(), nil)
		selector := selectorInit(ctx, nodes, filter)

		selected, err := selector(ctx, storj.NodeID{}, 3, nil, nil)
		require.NoError(t, err)

		require.Len(t, selected, 1)
		require.NotEqual(t, selected[0].CountryCode, location.Austria, "Should only select German nodes")
	})

	t.Run("with constraint and filter", func(t *testing.T) {
		nodes := []*nodeselection.SelectedNode{
			{ID: testrand.NodeID(), LastNet: "192.168.1.0/24", CountryCode: location.Germany},
			{ID: testrand.NodeID(), LastNet: "192.168.2.0/24", CountryCode: location.Austria},
			{ID: testrand.NodeID(), LastNet: "192.168.1.0/24", CountryCode: location.Germany},
		}

		filter := nodeselection.NodeFilterFunc(func(node *nodeselection.SelectedNode) bool {
			return node.CountryCode == location.Germany
		})

		subnetAttr, err := nodeselection.CreateNodeAttribute("last_net")
		require.NoError(t, err)

		selectorInit := nodeselection.Reduce(
			nodeselection.RandomSelector(),
			nil,
			nodeselection.AtLeast(subnetAttr, 1), // Include while count <= 1 per subnet
		)
		selector := selectorInit(ctx, nodes, filter)

		selected, err := selector(ctx, storj.NodeID{}, 3, nil, nil)
		require.NoError(t, err)

		require.Len(t, selected, 1)

		for _, node := range selected {
			require.Equal(t, location.Germany, node.CountryCode)
		}
	})
}

func TestReduceConfigExpression(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Test that parses the config expression but documents the issue
	t.Run("config expression parsing", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 15; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
				Tags: nodeselection.NodeTags{
					{
						Name:  "server_name",
						Value: []byte("server" + string(rune('A'+i/5))), // 3 servers: A, B, C (5 nodes each)
					},
				},
			})
		}

		environment := nodeselection.NewPlacementConfigEnvironment(nil, nil)

		// This expression should parse without error
		selectorInit, err := nodeselection.SelectorFromString(
			`reduce(random(), node_value("free_disk") * -1, atleast(node_attribute("tag:server_name"), 10))`,
			environment,
		)
		require.NoError(t, err, "Expression should parse successfully")

		selector := selectorInit(ctx, nodes, nil)
		selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err, "Selector should execute without error")

		require.Len(t, selected, 10)
	})

	t.Run("equivalent working expression", func(t *testing.T) {
		// Show how to write a working version using a custom needMore function
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 15; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
				Tags: nodeselection.NodeTags{
					{
						Name:  "server_name",
						Value: []byte("server" + string(rune('A'+i/5))),
					},
				},
			})
		}

		attr, err := nodeselection.CreateNodeAttribute("tag:server_name")
		require.NoError(t, err)

		selectorInit := nodeselection.Reduce(
			nodeselection.RandomSelector(),
			nil,
			nodeselection.AtLeast(attr, 3), // Include until we have 3 different server groups
		)
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 15, nil, nil)
		require.NoError(t, err)

		// Should include nodes until we have 3 different servers (at least 11 nodes: 5 serverA + 5 serverB + 1 serverC)
		require.GreaterOrEqual(t, len(selected), 11, "Should include nodes until we have 3 different server groups")

		// Verify we have nodes from 3 different servers
		serverCounts := make(map[string]int)
		for _, node := range selected {
			serverName := ""
			for _, tag := range node.Tags {
				if tag.Name == "server_name" {
					serverName = string(tag.Value)
					break
				}
			}
			if serverName != "" {
				serverCounts[serverName]++
			}
		}

		require.Len(t, serverCounts, 3, "Should have nodes from 3 different servers")
		require.Contains(t, serverCounts, "serverA", "Should include serverA")
		require.Contains(t, serverCounts, "serverB", "Should include serverB")
		require.Contains(t, serverCounts, "serverC", "Should include serverC")
	})
}

func TestReduceSortOrder(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("sort order with node_value free_disk", func(t *testing.T) {
		// Create nodes with different free disk values and different subnets
		// This will test that the sort order determines which nodes are processed first
		nodes := []*nodeselection.SelectedNode{
			{ID: testrand.NodeID(), FreeDisk: 1000000, LastNet: "192.168.1.0/24"}, // 1MB
			{ID: testrand.NodeID(), FreeDisk: 5000000, LastNet: "192.168.2.0/24"}, // 5MB
			{ID: testrand.NodeID(), FreeDisk: 2000000, LastNet: "192.168.3.0/24"}, // 2MB
			{ID: testrand.NodeID(), FreeDisk: 8000000, LastNet: "192.168.4.0/24"}, // 8MB
			{ID: testrand.NodeID(), FreeDisk: 3000000, LastNet: "192.168.5.0/24"}, // 3MB
		}

		// Create a sort order based on free_disk (descending - higher free disk first)
		freeDiskValue, err := nodeselection.CreateNodeValue("free_disk")
		require.NoError(t, err)

		sortOrder := nodeselection.Compare(nodeselection.Desc(nodeselection.ScoreNodeFunc(func(uplink storj.NodeID, node *nodeselection.SelectedNode) float64 {
			return freeDiskValue(*node)
		})))

		subnetAttr, err := nodeselection.CreateNodeAttribute("last_net")
		require.NoError(t, err)

		// Use Reduce with the sort order - should process nodes in descending order of free disk
		// Since nodes have different subnets, AtLeast(3) will select until 3 different subnets are found
		selectorInit := nodeselection.Reduce(
			nodeselection.RandomSelector(),
			sortOrder,
			nodeselection.AtLeast(subnetAttr, 3), // Include until 3 different subnets
		)
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)

		// Should select exactly 3 nodes (due to the AtLeast constraint)
		require.Len(t, selected, 3)

		// Verify that the 3 nodes with highest free disk are selected
		// Sort the selected nodes by FreeDisk descending to verify order
		selectedFreeDisk := make([]int64, len(selected))
		for i, node := range selected {
			selectedFreeDisk[i] = node.FreeDisk
		}

		// The top 3 should be 8MB, 5MB, 3MB in some order
		require.Contains(t, selectedFreeDisk, int64(8000000), "Should include node with 8MB")
		require.Contains(t, selectedFreeDisk, int64(5000000), "Should include node with 5MB")
		require.Contains(t, selectedFreeDisk, int64(3000000), "Should include node with 3MB")

		// Should not include the lower values
		require.NotContains(t, selectedFreeDisk, int64(1000000), "Should not include node with 1MB")
		require.NotContains(t, selectedFreeDisk, int64(2000000), "Should not include node with 2MB")
	})

	t.Run("sort order affects selection with different subnets", func(t *testing.T) {
		// Create nodes where sort order matters for selection across different subnets
		nodes := []*nodeselection.SelectedNode{
			{ID: testrand.NodeID(), FreeDisk: 1000000, LastNet: "192.168.1.0/24"}, // 1MB - subnet1 (lower priority)
			{ID: testrand.NodeID(), FreeDisk: 8000000, LastNet: "192.168.1.0/24"}, // 8MB - subnet1 (should be selected first)
			{ID: testrand.NodeID(), FreeDisk: 3000000, LastNet: "192.168.1.0/24"}, // 3MB - subnet1
			{ID: testrand.NodeID(), FreeDisk: 2000000, LastNet: "192.168.2.0/24"}, // 2MB - subnet2 (lower priority)
			{ID: testrand.NodeID(), FreeDisk: 5000000, LastNet: "192.168.2.0/24"}, // 5MB - subnet2 (should be selected first)
		}

		freeDiskValue, err := nodeselection.CreateNodeValue("free_disk")
		require.NoError(t, err)

		// Sort by free_disk descending (highest first)
		sortOrder := nodeselection.Compare(nodeselection.Desc(nodeselection.ScoreNodeFunc(func(uplink storj.NodeID, node *nodeselection.SelectedNode) float64 {
			return freeDiskValue(*node)
		})))

		// Custom needMore function that stops after we've seen at least one node from each of the two subnets
		var seenSubnets map[string]bool
		needMoreFunc := func() func(node *nodeselection.SelectedNode) bool {
			seenSubnets = make(map[string]bool)
			return func(node *nodeselection.SelectedNode) bool {
				subnet := node.LastNet
				seenSubnets[subnet] = true
				// Continue while we haven't seen both subnets yet
				return len(seenSubnets) < 2
			}
		}

		// Use Reduce with constraint that ensures we get at least one node from each subnet
		selectorInit := nodeselection.Reduce(
			nodeselection.RandomSelector(),
			sortOrder,
			needMoreFunc, // Custom logic for cross-subnet selection
		)
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)

		// Should select 2 nodes (one from each subnet, the highest FreeDisk from each)
		require.Len(t, selected, 2)

		// Verify that the nodes with highest free disk from each subnet are selected
		subnetToFreeDisk := make(map[string]int64)
		for _, node := range selected {
			subnetToFreeDisk[node.LastNet] = node.FreeDisk
		}

		// Should have selected the 8MB node from subnet 1 and 5MB node from subnet 2
		// because the sort order processes nodes by descending free disk
		require.Equal(t, int64(8000000), subnetToFreeDisk["192.168.1.0/24"], "Should select node with highest free disk from subnet 1")
		require.Equal(t, int64(5000000), subnetToFreeDisk["192.168.2.0/24"], "Should select node with highest free disk from subnet 2")
	})
}

func TestDailyPeriods(t *testing.T) {
	require.Equal(t, int64(1), nodeselection.DailyPeriodsForHour(1, []int64{1, 2}))
	require.Equal(t, int64(1), nodeselection.DailyPeriodsForHour(11, []int64{1, 2}))
	require.Equal(t, int64(2), nodeselection.DailyPeriodsForHour(12, []int64{1, 2}))
	require.Equal(t, int64(2), nodeselection.DailyPeriodsForHour(23, []int64{1, 2}))

	require.Equal(t, int64(4), nodeselection.DailyPeriodsForHour(23, []int64{1, 2, 3, 4}))
}

func TestMultiSelector(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("combines multiple selectors", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 20; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID:         testrand.NodeID(),
				LastNet:    fmt.Sprintf("192.168.%d.0/24", i/5),
				LastIPPort: fmt.Sprintf("192.168.%d.%d:8080", i/5, i%5+1),
			})
		}

		// Create multi-selector that combines random selector with itself
		selectorInit := nodeselection.MultiSelector(
			nodeselection.RandomSelector(),
			nodeselection.RandomSelector(),
		)
		selector := selectorInit(ctx, nodes, nil)

		// Request 10 nodes, each selector should get 5
		selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 10)

		// Note: MultiSelector doesn't prevent duplicates between selectors,
		// so we just verify we got the expected number of nodes
		require.LessOrEqual(t, len(selected), 20) // Can't exceed available nodes
	})

	t.Run("distributes nodes evenly among selectors", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 30; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testrand.NodeID(),
			})
		}

		// Create multi-selector with 3 random selectors
		selectorInit := nodeselection.MultiSelector(
			nodeselection.RandomSelector(),
			nodeselection.RandomSelector(),
			nodeselection.RandomSelector(),
		)
		selector := selectorInit(ctx, nodes, nil)

		// Request 15 nodes, each selector should get 5
		selected, err := selector(ctx, storj.NodeID{}, 15, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 15)
	})

	t.Run("handles empty selectors", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 10; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testrand.NodeID(),
			})
		}

		selectorInit := nodeselection.MultiSelector()
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 0)
	})

	t.Run("handles single selector", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 10; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testrand.NodeID(),
			})
		}

		selectorInit := nodeselection.MultiSelector(
			nodeselection.RandomSelector(),
		)
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 5, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 5)
	})

	t.Run("basic functionality with insufficient nodes", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 4; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testrand.NodeID(),
			})
		}

		// Create multi-selector with 2 random selectors
		selectorInit := nodeselection.MultiSelector(
			nodeselection.RandomSelector(),
			nodeselection.RandomSelector(),
		)
		selector := selectorInit(ctx, nodes, nil)

		// Request 10 nodes total, each selector gets 5
		// With only 4 nodes available, each selector can return at most 4 nodes
		selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		// Could get duplicates between selectors, so length could vary
		require.GreaterOrEqual(t, len(selected), 0)
		require.LessOrEqual(t, len(selected), 8) // At most 4 nodes from each selector
	})
}

func TestFixedSelector(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("overrides requested count with fixed count", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 20; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID: testrand.NodeID(),
			})
		}

		// Create fixed selector that always selects 7 nodes
		selectorInit := nodeselection.FixedSelector(7, nodeselection.RandomSelector())
		selector := selectorInit(ctx, nodes, nil)

		// Request 10 nodes but should only get 7
		selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 7)

		// Request 3 nodes but should still get 7
		selected, err = selector(ctx, storj.NodeID{}, 3, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 7)
	})

	t.Run("delegates to wrapped selector correctly", func(t *testing.T) {
		var nodes []*nodeselection.SelectedNode
		for i := 0; i < 20; i++ {
			nodes = append(nodes, &nodeselection.SelectedNode{
				ID:      testrand.NodeID(),
				LastNet: fmt.Sprintf("192.168.%d.0/24", i),
			})
		}

		// Use attribute group selector as delegate to verify delegation
		attribute, err := nodeselection.CreateNodeAttribute("last_net")
		require.NoError(t, err)

		selectorInit := nodeselection.FixedSelector(5, nodeselection.AttributeGroupSelector(attribute))
		selector := selectorInit(ctx, nodes, nil)

		selected, err := selector(ctx, storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selected, 5)

		// Verify the delegate's behavior is preserved (different subnets)
		subnets := make(map[string]bool)
		for _, node := range selected {
			subnets[node.LastNet] = true
		}
		require.Equal(t, 5, len(subnets), "Each selected node should be from different subnet")
	})
}
