// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeselection"
)

func TestDownloadSwitch(t *testing.T) {
	ctx := testcontext.New(t)

	// Create test nodes
	node1 := &nodeselection.SelectedNode{ID: testrand.NodeID()}
	node2 := &nodeselection.SelectedNode{ID: testrand.NodeID()}
	node3 := &nodeselection.SelectedNode{ID: testrand.NodeID()}
	node4 := &nodeselection.SelectedNode{ID: testrand.NodeID()}

	possibleNodes := map[storj.NodeID]*nodeselection.SelectedNode{
		node1.ID: node1,
		node2.ID: node2,
		node3.ID: node3,
		node4.ID: node4,
	}

	requester := testrand.NodeID()

	t.Run("no cases - uses default selector", func(t *testing.T) {
		defaultSelector := func(ctx context.Context, requester storj.NodeID, nodes map[storj.NodeID]*nodeselection.SelectedNode, needed int) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
			result := make(map[storj.NodeID]*nodeselection.SelectedNode)
			count := 0
			for id, node := range nodes {
				if count >= needed {
					break
				}
				result[id] = node
				count++
			}
			return result, nil
		}

		selector := nodeselection.DownloadSwitch(defaultSelector)
		selected, err := selector(ctx, requester, possibleNodes, 2)

		require.NoError(t, err)
		assert.Len(t, selected, 2)
	})

	t.Run("case condition matches - uses case selector", func(t *testing.T) {
		matchingCondition := func(ctx context.Context, requestor storj.NodeID) bool {
			return requestor == requester
		}

		caseSelector := func(ctx context.Context, requester storj.NodeID, nodes map[storj.NodeID]*nodeselection.SelectedNode, needed int) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
			// Select only node1
			result := map[storj.NodeID]*nodeselection.SelectedNode{
				node1.ID: node1,
			}
			return result, nil
		}

		defaultSelector := func(ctx context.Context, requester storj.NodeID, nodes map[storj.NodeID]*nodeselection.SelectedNode, needed int) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
			// Select remaining nodes
			result := make(map[storj.NodeID]*nodeselection.SelectedNode)
			count := 0
			for id, node := range nodes {
				if count >= needed {
					break
				}
				result[id] = node
				count++
			}
			return result, nil
		}

		switchCase := nodeselection.NewDownloadCase(matchingCondition, caseSelector)
		selector := nodeselection.DownloadSwitch(defaultSelector, switchCase)
		selected, err := selector(ctx, requester, possibleNodes, 3)

		require.NoError(t, err)
		assert.Len(t, selected, 3)
		assert.Contains(t, selected, node1.ID, "Should contain node1 from case selector")
	})

	t.Run("case condition doesn't match - uses default selector", func(t *testing.T) {
		nonMatchingCondition := func(ctx context.Context, requestor storj.NodeID) bool {
			return requestor != requester // Will never match
		}

		caseSelector := func(ctx context.Context, requester storj.NodeID, nodes map[storj.NodeID]*nodeselection.SelectedNode, needed int) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
			// Should never be called
			t.Fatal("Case selector should not be called when condition doesn't match")
			return nil, nil
		}

		defaultSelector := func(ctx context.Context, requester storj.NodeID, nodes map[storj.NodeID]*nodeselection.SelectedNode, needed int) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
			result := make(map[storj.NodeID]*nodeselection.SelectedNode)
			count := 0
			for id, node := range nodes {
				if count >= needed {
					break
				}
				result[id] = node
				count++
			}
			return result, nil
		}

		switchCase := nodeselection.NewDownloadCase(nonMatchingCondition, caseSelector)
		selector := nodeselection.DownloadSwitch(defaultSelector, switchCase)
		selected, err := selector(ctx, requester, possibleNodes, 2)

		require.NoError(t, err)
		assert.Len(t, selected, 2)
	})

	t.Run("multiple cases - stops when enough nodes selected", func(t *testing.T) {
		matchingCondition := func(ctx context.Context, requestor storj.NodeID) bool {
			return true
		}

		case1Selector := func(ctx context.Context, requester storj.NodeID, nodes map[storj.NodeID]*nodeselection.SelectedNode, needed int) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
			return map[storj.NodeID]*nodeselection.SelectedNode{
				node1.ID: node1,
				node2.ID: node2,
			}, nil
		}

		case2Selector := func(ctx context.Context, requester storj.NodeID, nodes map[storj.NodeID]*nodeselection.SelectedNode, needed int) (map[storj.NodeID]*nodeselection.SelectedNode, error) {
			t.Fatal("Second case should not be called when first case satisfies needed count")
			return nil, nil
		}

		defaultSelector := nodeselection.DefaultDownloadSelector

		case1 := nodeselection.NewDownloadCase(matchingCondition, case1Selector)
		case2 := nodeselection.NewDownloadCase(matchingCondition, case2Selector)
		selector := nodeselection.DownloadSwitch(defaultSelector, case1, case2)
		selected, err := selector(ctx, requester, possibleNodes, 2)

		require.NoError(t, err)
		assert.Len(t, selected, 2)
		assert.Contains(t, selected, node1.ID)
		assert.Contains(t, selected, node2.ID)
	})
}

func TestDownloadFilter(t *testing.T) {
	ctx := testcontext.New(t)

	// Create test nodes with different properties
	node1 := &nodeselection.SelectedNode{
		ID:      testrand.NodeID(),
		LastNet: "192.168.1.1",
		Email:   "test1@example.com",
		Wallet:  "wallet1",
		Vetted:  true,
		Exiting: false,
	}
	node2 := &nodeselection.SelectedNode{
		ID:      testrand.NodeID(),
		LastNet: "192.168.1.2",
		Email:   "test2@example.com",
		Wallet:  "wallet2",
		Vetted:  false,
		Exiting: false,
	}
	node3 := &nodeselection.SelectedNode{
		ID:      testrand.NodeID(),
		LastNet: "192.168.1.3",
		Email:   "test3@example.com",
		Wallet:  "wallet3",
		Vetted:  true,
		Exiting: true,
	}

	possibleNodes := map[storj.NodeID]*nodeselection.SelectedNode{
		node1.ID: node1,
		node2.ID: node2,
		node3.ID: node3,
	}

	requester := testrand.NodeID()

	t.Run("filter allows all nodes", func(t *testing.T) {
		allowAllFilter := nodeselection.NodeFilterFunc(func(node *nodeselection.SelectedNode) bool {
			return true
		})

		selector := nodeselection.DownloadFilter(allowAllFilter, nodeselection.DefaultDownloadSelector)
		selected, err := selector(ctx, requester, possibleNodes, 3)

		require.NoError(t, err)
		assert.Len(t, selected, 3)
	})

	t.Run("filter blocks all nodes", func(t *testing.T) {
		blockAllFilter := nodeselection.NodeFilterFunc(func(node *nodeselection.SelectedNode) bool {
			return false
		})

		selector := nodeselection.DownloadFilter(blockAllFilter, nodeselection.DefaultDownloadSelector)
		selected, err := selector(ctx, requester, possibleNodes, 3)

		require.NoError(t, err)
		assert.Len(t, selected, 0)
	})

	t.Run("filter allows only vetted nodes", func(t *testing.T) {
		vettedFilter := nodeselection.NodeFilterFunc(func(node *nodeselection.SelectedNode) bool {
			return node.Vetted
		})

		selector := nodeselection.DownloadFilter(vettedFilter, nodeselection.DefaultDownloadSelector)
		selected, err := selector(ctx, requester, possibleNodes, 3)

		require.NoError(t, err)
		assert.Len(t, selected, 2) // node1 and node3 are vetted
		assert.Contains(t, selected, node1.ID)
		assert.Contains(t, selected, node3.ID)
		assert.NotContains(t, selected, node2.ID)
	})

}

func TestRequesterIs(t *testing.T) {
	ctx := testcontext.New(t)

	target1 := testrand.NodeID()
	target2 := testrand.NodeID()
	other := testrand.NodeID()

	t.Run("single target - matches", func(t *testing.T) {
		condition := nodeselection.RequesterIs(target1.String())
		result := condition(ctx, target1)
		assert.True(t, result)
	})

	t.Run("single target - doesn't match", func(t *testing.T) {
		condition := nodeselection.RequesterIs(target1.String())
		result := condition(ctx, other)
		assert.False(t, result)
	})

	t.Run("multiple targets - matches first", func(t *testing.T) {
		condition := nodeselection.RequesterIs(target1.String(), target2.String())
		result := condition(ctx, target1)
		assert.True(t, result)
	})

	t.Run("multiple targets - matches second", func(t *testing.T) {
		condition := nodeselection.RequesterIs(target1.String(), target2.String())
		result := condition(ctx, target2)
		assert.True(t, result)
	})

	t.Run("multiple targets - doesn't match any", func(t *testing.T) {
		condition := nodeselection.RequesterIs(target1.String(), target2.String())
		result := condition(ctx, other)
		assert.False(t, result)
	})

	t.Run("no targets", func(t *testing.T) {
		condition := nodeselection.RequesterIs()
		result := condition(ctx, target1)
		assert.False(t, result)
	})
}
