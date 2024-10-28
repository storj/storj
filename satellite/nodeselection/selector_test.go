// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection_test

import (
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
	selector := nodeselection.RandomSelector()(nodes, nil)

	const (
		reqCount       = 2
		executionCount = 10000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 2 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(storj.NodeID{}, reqCount, nil, nil)
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
	selector := nodeselection.AttributeGroupSelector(attribute)(nodes, nil)

	const (
		reqCount       = 2
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 2 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(storj.NodeID{}, reqCount, nil, nil)
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
	selector := nodeselection.AttributeGroupSelector(attribute)(nodes, nil)

	const (
		reqCount       = 1
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 1 node
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(storj.NodeID{}, reqCount, nil, nil)
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

	selector := nodeselection.RandomSelector()(nodes, nil)
	selected, err := selector(storj.NodeID{}, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, selected, 3)
	selected, err = selector(storj.NodeID{}, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, selected, 3)

	selector = nodeselection.RandomSelector()(nodes, nodeselection.NodeFilters{}.WithExcludedIDs([]storj.NodeID{firstID, secondID}))
	selected, err = selector(storj.NodeID{}, 3, nil, nil)
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
	selector := nodeselection.AttributeGroupSelector(attribute)(nodes, filter)
	for i := 0; i < 100; i++ {
		selected, err := selector(storj.NodeID{}, 4, nil, nil)
		require.NoError(t, err)
		assert.Len(t, selected, 4)
	}
}

func TestFilterSelector(t *testing.T) {
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

	initialized := selector(nodes, nil)
	for i := 0; i < 100; i++ {
		selected, err := initialized(storj.NodeID{}, 3, []storj.NodeID{}, nil)
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
	selector := nodeselection.BalancedGroupBasedSelector(attribute, nil)(nodes, nil)

	var badSelection atomic.Int64
	for i := 0; i < 1000; i++ {
		ctx.Go(func() error {
			selectedNodes, err := selector(storj.NodeID{}, 10, nil, nil)
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

	selector := nodeselection.BalancedGroupBasedSelector(attribute, nil)(nodes, nil)

	histogram := map[string]int{}
	for i := 0; i < 1000; i++ {
		selectedNodes, err := selector(storj.NodeID{}, 7, excluded, alreadySelected)
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
		selector := selectorInit(nodes[:10], nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 10)
			require.Equal(t, 0, countUnvetted(selected))
		}
	})

	t.Run("25% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0.25, nodeselection.RandomSelector())
		selector := selectorInit(nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 5)
			require.Equal(t, 1, countUnvetted(selected))
		}
	})

	t.Run("15% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0.15, nodeselection.RandomSelector())
		selector := selectorInit(nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			// The faction result in less than 1 node, so it randonly decide if 0 or 1 vetted node is
			// selected.
			require.InDelta(t, 0, countUnvetted(selected), 1)
		}
	})

	t.Run("0.01% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0.0001, nodeselection.RandomSelector())
		selector := selectorInit(nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			// The faction result in less than 1 node, so it randonly decide if 0 or 1 vetted node is
			// selected.
			require.InDelta(t, 0, countUnvetted(selected), 1)
		}
	})

	t.Run("0% of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(0, nodeselection.RandomSelector())
		selector := selectorInit(nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Zero(t, countUnvetted(selected))
		}
	})

	t.Run("negative % of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(-1, nodeselection.RandomSelector())
		selector := selectorInit(nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Zero(t, countUnvetted(selected))
		}
	})

	t.Run("NaN % of 5", func(t *testing.T) {
		selectorInit := nodeselection.UnvettedSelector(math.NaN(), nodeselection.RandomSelector())
		selector := selectorInit(nodes, nil)

		for i := 0; i < 100; i++ {
			selected, err := selector(storj.NodeID{}, 5, nil, nil)
			require.NoError(t, err)
			require.Zero(t, countUnvetted(selected))
		}
	})
}

func TestChoiceOfTwo(t *testing.T) {
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

	selector := nodeselection.ChoiceOfTwo(tracker, nodeselection.RandomSelector())
	initializedSelector := selector(nodes, nil)

	for i := 0; i < 100; i++ {
		selectedNodes, err := initializedSelector(tracker.trustedUplink, 10, nil, nil)
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
		selectedNodes, err := initializedSelector(storj.NodeID{}, 10, nil, nil)
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

	selector := nodeselection.ChoiceOfN(tracker, 3, nodeselection.RandomSelector())
	initializedSelector := selector(nodes, nil)

	for i := 0; i < 100; i++ {
		selectedNodes, err := initializedSelector(tracker.trustedUplink, 10, nil, nil)
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
		selectedNodes, err := initializedSelector(storj.NodeID{}, 10, nil, nil)
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
			nodeSelector := selectorInit(nodes, nil)
			for i := 0; i < 100; i++ {
				selected, err := nodeSelector(storj.NodeID{}, 8, nil, nil)
				require.NoError(t, err)
				require.Len(t, selected, 8)
				require.Equal(t, 0, countSlowNodes(selected))
			}
		}
	})

	t.Run("keep best 8", func(t *testing.T) {
		selectorInit := nodeselection.FilterBest(tracker, "8", "", nodeselection.RandomSelector())
		nodeSelector := selectorInit(nodes, nil)
		for i := 0; i < 10; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 2, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 2)
			require.Equal(t, 0, countSlowNodes(selected))
		}
	})

	t.Run("cut off worst 30", func(t *testing.T) {
		selectorInit := nodeselection.FilterBest(tracker, "-30", "", nodeselection.RandomSelector())
		nodeSelector := selectorInit(nodes, nil)
		for i := 0; i < 10; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 0)
			require.Equal(t, 0, countSlowNodes(selected))
		}
	})
}

func TestFilterBestOfN(t *testing.T) {
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
		nodeSelector := selectorInit(nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 10, nil, nil)
			require.NoError(t, err)
			require.Len(t, selected, 10)
			require.Equal(t, 0, countSlowNodes(selected))
		}
	})

	t.Run("fastest 10 out of 5", func(t *testing.T) {
		selectorInit := nodeselection.BestOfN(tracker, 0.5, nodeselection.RandomSelector())
		nodeSelector := selectorInit(nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 10, nil, nil)
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
		nodeselection.EqualSelector(surgeTag, "true"), lastIpPortAttribute, lastNetAttribute), nil)(nodes, nil)

	const (
		reqCount       = 3
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 3 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes, err := selector(storj.NodeID{}, reqCount, nil, nil)
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

	slowFilter, err := nodeselection.NewAttributeFilter("email", "slow")
	require.NoError(t, err)
	fastFilter, err := nodeselection.NewAttributeFilter("email", "fast")
	require.NoError(t, err)

	t.Run("3 from slow, 7 from remaining", func(t *testing.T) {
		nodes, _ := generateNodes(10, 10)

		selectorInit := nodeselection.DualSelector(
			0.3,
			nodeselection.FilteredSelector(slowFilter, nodeselection.RandomSelector()),
			nodeselection.FilteredSelector(fastFilter, nodeselection.RandomSelector()),
		)
		nodeSelector := selectorInit(nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 10, nil, nil)
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
		nodeSelector := selectorInit(nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 10, nil, nil)
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
		nodeSelector := selectorInit(nodes, nil)
		for i := 0; i < 100; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 10, nil, nil)
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
		nodeSelector := selectorInit(nodes, nil)
		slowCounts := 0
		allCounts := 0
		for i := 0; i < 1000; i++ {
			selected, err := nodeSelector(storj.NodeID{}, 10, nil, nil)
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
	predictableSelector := func(nodes []*nodeselection.SelectedNode, filter nodeselection.NodeFilter) nodeselection.NodeSelector {
		return func(requester storj.NodeID, n int, excluded []storj.NodeID, alreadySelected []*nodeselection.SelectedNode) ([]*nodeselection.SelectedNode, error) {
			ix++
			return selections[ix], nil
		}
	}
	selector := nodeselection.ChoiceOfNSelection(3, predictableSelector, nodeselection.LastBut(nodeselection.Desc(nodeselection.PieceCount(10)), 0))
	initializedSelector := selector(nil, nil)
	selection, err := initializedSelector(storj.NodeID{}, 10, nil, nil)
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
		"min":     nodeselection.Min,
		"tracker": tracker,
	}
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
	val, err := nodeselection.CreateNodeValue("tag:1111111111111111111111111111111112m1s9K/weight")
	require.NoError(t, err)

	selector := nodeselection.WeightedSelector(val, 100, nil)(nodes, nil)

	histogram := map[storj.NodeID]int{}

	for i := 0; i < 10000; i++ {
		selectedNodes, err := selector(storj.NodeID{}, 10, nil, nil)
		require.NoError(t, err)
		require.Len(t, selectedNodes, 10)

		for _, node := range selectedNodes {
			histogram[node.ID]++
		}
	}

	// specific node selected at least 3 times more
	require.Greater(t, float64(histogram[nodes[0].ID])/float64(histogram[nodes[1].ID]), float64(3))

}
