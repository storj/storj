// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
)

func TestGroupConstraint(t *testing.T) {
	attribute := func(node SelectedNode) string {
		return node.LastNet
	}

	constraint := GroupConstraint(attribute, 2)

	nodes := []*SelectedNode{
		{ID: storj.NodeID{1}, LastNet: "net1"},
		{ID: storj.NodeID{2}, LastNet: "net2"},
		{ID: storj.NodeID{3}, LastNet: "net2"},
	}

	assert.True(t, constraint(nodes, &SelectedNode{ID: storj.NodeID{3}, LastNet: "net1"}))
	assert.False(t, constraint(nodes, &SelectedNode{ID: storj.NodeID{3}, LastNet: "net2"}))

}

func TestStreamFilter(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create test nodes
	nodes := []*SelectedNode{
		{ID: storj.NodeID{1}, LastNet: "net1"},
		{ID: storj.NodeID{2}, LastNet: "net1"},
		{ID: storj.NodeID{3}, LastNet: "net2"},
		{ID: storj.NodeID{4}, LastNet: "net2"},
		{ID: storj.NodeID{5}, LastNet: "net3"},
	}

	// Create a simple stream that returns nodes in order
	baseStream := func(ctx context.Context, requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
		i := 0
		return func(ctx context.Context) *SelectedNode {
			if i >= len(nodes) {
				return nil
			}
			node := nodes[i]
			i++
			return node
		}
	}

	// Create a filter that rejects nodes with LastNet="net1"
	filter := func(selected []*SelectedNode, node *SelectedNode) bool {
		return node.LastNet != "net1" // Return true to include, false to exclude
	}

	// Apply the filter
	filteredStream := StreamFilter(filter)(baseStream)

	// Test the filtered stream
	sequence := filteredStream(ctx, storj.NodeID{}, nil, nil)

	// We should get nodes 3, 4, and 5 (with LastNet != "net1")
	node := sequence(ctx)
	require.NotNil(t, node)
	assert.Equal(t, storj.NodeID{3}, node.ID)

	node = sequence(ctx)
	require.NotNil(t, node)
	assert.Equal(t, storj.NodeID{4}, node.ID)

	node = sequence(ctx)
	require.NotNil(t, node)
	assert.Equal(t, storj.NodeID{5}, node.ID)

	// No more nodes
	node = sequence(ctx)
	assert.Nil(t, node)
}

func TestStream(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create test nodes
	allNodes := []*SelectedNode{
		{ID: storj.NodeID{1}},
		{ID: storj.NodeID{2}},
		{ID: storj.NodeID{3}},
		{ID: storj.NodeID{4}},
		{ID: storj.NodeID{5}},
	}

	// Create a simple seed function
	seed := func(nodes []*SelectedNode) NodeStream {
		return func(ctx context.Context, requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
			i := 0
			return func(ctx context.Context) *SelectedNode {
				if i >= len(nodes) {
					return nil
				}
				node := nodes[i]
				i++
				return node
			}
		}
	}

	// Create a selector
	selector := Stream(seed)

	// Initialize the selector with all nodes and no filter
	nodeSelector := selector(ctx, allNodes, nil)

	// Test selecting 3 nodes
	selected, err := nodeSelector(ctx, storj.NodeID{}, 3, nil, nil)
	require.NoError(t, err)
	require.Len(t, selected, 3)
	assert.Equal(t, storj.NodeID{1}, selected[0].ID)
	assert.Equal(t, storj.NodeID{2}, selected[1].ID)
	assert.Equal(t, storj.NodeID{3}, selected[2].ID)

	// Test with exclusions
	excluded := []storj.NodeID{{1}, {2}}
	selected, err = nodeSelector(ctx, storj.NodeID{}, 3, excluded, nil)
	require.NoError(t, err)
	require.Len(t, selected, 3)
	assert.Equal(t, storj.NodeID{3}, selected[0].ID)
	assert.Equal(t, storj.NodeID{4}, selected[1].ID)
	assert.Equal(t, storj.NodeID{5}, selected[2].ID)

	// Test requesting more nodes than available
	_, err = nodeSelector(ctx, storj.NodeID{}, 6, nil, nil)
	require.Error(t, err)
}

func TestRandomStream(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create test nodes
	allNodes := []*SelectedNode{
		{ID: storj.NodeID{1}},
		{ID: storj.NodeID{2}},
		{ID: storj.NodeID{3}},
		{ID: storj.NodeID{4}},
		{ID: storj.NodeID{5}},
	}

	// Create a random stream
	stream := RandomStream(allNodes)
	sequence := stream(ctx, storj.NodeID{}, nil, nil)

	// Collect all nodes from the stream
	var selectedNodes []*SelectedNode
	for {
		node := sequence(ctx)
		if node == nil {
			break
		}
		selectedNodes = append(selectedNodes, node)
	}

	// We should get all nodes
	require.Len(t, selectedNodes, len(allNodes))

	// Test with exclusions
	excluded := []storj.NodeID{{1}, {3}}
	sequence = stream(ctx, storj.NodeID{}, excluded, nil)

	selectedNodes = nil
	for {
		node := sequence(ctx)
		if node == nil {
			break
		}
		selectedNodes = append(selectedNodes, node)
		// Verify excluded nodes are not selected
		assert.NotEqual(t, storj.NodeID{1}, node.ID)
		assert.NotEqual(t, storj.NodeID{3}, node.ID)
	}

	// We should get 3 nodes (5 total - 2 excluded)
	require.Len(t, selectedNodes, 3)
}

func TestChoiceOfNStream(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create test nodes
	allNodes := []*SelectedNode{
		{ID: storj.NodeID{1}, LastNet: "net1"},
		{ID: storj.NodeID{2}, LastNet: "net2"},
		{ID: storj.NodeID{3}, LastNet: "net3"},
		{ID: storj.NodeID{4}, LastNet: "net4"},
		{ID: storj.NodeID{5}, LastNet: "net5"},
	}

	// Create a simple base stream
	baseStream := func(ctx context.Context, requester storj.NodeID, excluded []storj.NodeID, alreadySelected []*SelectedNode) NodeSequence {
		i := 0
		return func(ctx context.Context) *SelectedNode {
			if i >= len(allNodes) {
				return nil
			}
			node := allNodes[i]
			i++
			return node
		}
	}

	// Create a score function that scores nodes by their ID value
	scoreNode := &testScoreNode{
		scoreFunc: func(node *SelectedNode) float64 {
			return float64(node.ID[0]) // Use first byte of ID as score
		},
	}

	choiceStream := ChoiceOfNStream(3, scoreNode)(baseStream)

	for i := 0; i < 100; i++ {
		sequence := choiceStream(ctx, storj.NodeID{}, nil, nil)
		node := sequence(ctx)
		require.NotNil(t, node)
		// even the worst case scenario (1,2,3 selected), the 3 is the best score
		assert.Greater(t, int(node.ID[0]), 2)
	}

}

// Helper types for testing

type testScoreNode struct {
	scoreFunc func(*SelectedNode) float64
}

func (t *testScoreNode) Get(id storj.NodeID) func(*SelectedNode) float64 {
	return t.scoreFunc
}
