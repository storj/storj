// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeselection"
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
	selector := nodeselection.BalancedGroupBasedSelector(attribute)(nodes, nil)

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

	selector := nodeselection.BalancedGroupBasedSelector(attribute)(nodes, nil)

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
			require.Equal(t, 0, countUnvetted(selected))
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

// mockSelector returns only 1 success, for slow nodes, but only if trustedUplink does ask it.
type mockTracker struct {
	trustedUplink storj.NodeID
	slowNodes     []storj.NodeID
}

func (m *mockTracker) Get(uplink storj.NodeID) func(node storj.NodeID) float64 {
	return func(node storj.NodeID) float64 {
		if uplink == m.trustedUplink {
			for _, slow := range m.slowNodes {
				if slow == node {
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
