// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uploadselection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeselection/uploadselection"
)

func TestSelectByID(t *testing.T) {
	// create 3 nodes, 2 with same subnet
	// perform many node selections that selects 2 nodes
	// expect that the all node are selected ~33% of the time.
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// create 3 nodes, 2 with same subnet
	lastNetDuplicate := "1.0.1"
	subnetA1 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}
	subnetA2 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	lastNetSingle := "1.0.2"
	subnetB1 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*uploadselection.SelectedNode{subnetA1, subnetA2, subnetB1}
	selector := uploadselection.SelectByID(nodes)

	const (
		reqCount       = 2
		executionCount = 10000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 2 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes := selector.Select(reqCount, uploadselection.NodeFilters{})
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
	subnetA1 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}
	subnetA2 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	lastNetSingle := "1.0.2"
	subnetB1 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*uploadselection.SelectedNode{subnetA1, subnetA2, subnetB1}
	selector := uploadselection.SelectBySubnetFromNodes(nodes)

	const (
		reqCount       = 2
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 2 nodes
	for i := 0; i < executionCount; i++ {
		selectedNodes := selector.Select(reqCount, uploadselection.NodeFilters{})
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
	subnetA1 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}
	subnetA2 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	lastNetSingle := "1.0.2"
	subnetB1 := &uploadselection.SelectedNode{
		ID:         testrand.NodeID(),
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*uploadselection.SelectedNode{subnetA1, subnetA2, subnetB1}
	selector := uploadselection.SelectBySubnetFromNodes(nodes)

	const (
		reqCount       = 1
		executionCount = 1000
	)

	var selectedNodeCount = map[storj.NodeID]int{}

	// perform many node selections that selects 1 node
	for i := 0; i < executionCount; i++ {
		selectedNodes := selector.Select(reqCount, uploadselection.NodeFilters{})
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
	subnetA1 := &uploadselection.SelectedNode{
		ID:         firstID,
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".4:8080",
	}

	secondID := testrand.NodeID()
	subnetA2 := &uploadselection.SelectedNode{
		ID:         secondID,
		LastNet:    lastNetDuplicate,
		LastIPPort: lastNetDuplicate + ".5:8080",
	}

	thirdID := testrand.NodeID()
	lastNetSingle := "1.0.2"
	subnetB1 := &uploadselection.SelectedNode{
		ID:         thirdID,
		LastNet:    lastNetSingle,
		LastIPPort: lastNetSingle + ".5:8080",
	}

	nodes := []*uploadselection.SelectedNode{subnetA1, subnetA2, subnetB1}
	selector := uploadselection.SelectByID(nodes)

	assert.Len(t, selector.Select(3, uploadselection.NodeFilters{}), 3)
	assert.Len(t, selector.Select(3, uploadselection.NodeFilters{}.WithAutoExcludeSubnets()), 2)
	assert.Len(t, selector.Select(3, uploadselection.NodeFilters{}), 3)

	assert.Len(t, selector.Select(3, uploadselection.NodeFilters{}.WithExcludedIDs([]storj.NodeID{firstID, secondID})), 1)
	assert.Len(t, selector.Select(3, uploadselection.NodeFilters{}.WithAutoExcludeSubnets()), 2)
	assert.Len(t, selector.Select(3, uploadselection.NodeFilters{}.WithExcludedIDs([]storj.NodeID{thirdID}).WithAutoExcludeSubnets()), 1)
}
