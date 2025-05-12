// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"math"
	"testing"

	"github.com/jtolio/mito"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/location"
)

func TestParsedConfig(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	config, err := LoadConfig("config_test.yaml", NewPlacementConfigEnvironment(mockTracker{}, nil))
	require.NoError(t, err)
	require.Len(t, config, 15)

	{
		// checking filters
		require.True(t, config[1].NodeFilter.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))
		require.False(t, config[1].NodeFilter.Match(&SelectedNode{
			CountryCode: location.Russia,
		}))
		require.Equal(t, "eu-1", config[1].Name)
	}

	{
		// checking upload filters

		// normal filter
		require.True(t, config[13].NodeFilter.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))
		require.True(t, config[13].NodeFilter.Match(&SelectedNode{
			CountryCode: location.Austria,
		}))

		// upload filter (further excludes DE)
		require.False(t, config[13].UploadFilter.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))
		require.True(t, config[13].UploadFilter.Match(&SelectedNode{
			CountryCode: location.Austria,
		}))

	}

	{
		// checking one invariant
		node := func(ix int, owner string) SelectedNode {
			return SelectedNode{
				ID: testidentity.MustPregeneratedSignedIdentity(ix, storj.LatestIDVersion()).ID,
				Tags: NodeTags{
					{
						Name:  "owner",
						Value: []byte(owner),
					},
				},
			}
		}

		piece := func(ix int, nodeIx int) metabase.Piece {
			return metabase.Piece{
				Number: uint16(ix), StorageNode: testidentity.MustPregeneratedSignedIdentity(nodeIx, storj.LatestIDVersion()).ID,
			}
		}

		result := config[0].Invariant(
			metabase.Pieces{
				piece(1, 1),
				piece(3, 2),
				piece(5, 3),
				piece(9, 4),
				piece(10, 5),
				piece(11, 6),
			},
			[]SelectedNode{
				node(1, "dery"),
				node(2, "blathy"),
				node(3, "blathy"),
				node(4, "zipernowsky"),
				node(5, "zipernowsky"),
				node(6, "zipernowsky"),
			})

		// last zipernowsky is too much, as we allow only 2
		require.Equal(t, 1, result.Count())
	}

	{
		// checking a selector
		selected, err := config[0].Selector(ctx, []*SelectedNode{
			{
				Vetted: false,
			},
		}, nil)(ctx, storj.NodeID{}, 1, nil, nil)

		// having: new, requires: 0% unvetted = 100% vetted
		require.Len(t, selected, 0)
		require.NoError(t, err)
	}

	{
		// smoketest for creating choice of two selector
		selected, err := config[2].Selector(ctx,
			[]*SelectedNode{
				{
					ID: testrand.NodeID(),
				},
				{
					ID: testrand.NodeID(),
				},
				{
					ID: testrand.NodeID(),
				},
			}, nil,
		)(ctx, storj.NodeID{}, 1, nil, nil)

		require.Len(t, selected, 1)
		require.NoError(t, err)
	}
}

func TestParsedConfigWithoutTracker(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// tracker is not available for certain microservices (like repair). Still the placement should work.
	config, err := LoadConfig("config_test.yaml", NewPlacementConfigEnvironment(nil, nil))
	require.NoError(t, err)
	require.Len(t, config, 15)

	// smoketest for creating choice of two selector
	selected, err := config[2].Selector(ctx,
		[]*SelectedNode{
			{
				ID: testrand.NodeID(),
			},
			{
				ID: testrand.NodeID(),
			},
			{
				ID: testrand.NodeID(),
			},
		}, nil,
	)(ctx, storj.NodeID{}, 1, nil, nil)

	require.Len(t, selected, 1)
	require.NoError(t, err)

}

func TestFilterFromString(t *testing.T) {
	filter, err := FilterFromString(`exclude(nodelist("filter_testdata.txt"))`, NewPlacementConfigEnvironment(nil, nil))
	require.NoError(t, err)

	require.False(t, filter.Match(&SelectedNode{
		ID: testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID,
	}))
	require.True(t, filter.Match(&SelectedNode{
		ID: testidentity.MustPregeneratedIdentity(3, storj.LatestIDVersion()).ID,
	}))

}

func TestSelectorFromString(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	selector, err := SelectorFromString(`filter(exclude(nodelist("filter_testdata.txt")),random())`, nil)
	require.NoError(t, err)

	// initialize the node space
	var nodes []*SelectedNode
	for i := 0; i < 10; i++ {
		nodes = append(nodes, &SelectedNode{
			ID: testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
		})
	}

	initialized := selector(ctx, nodes, nil)

	for i := 0; i < 100; i++ {
		selected, err := initialized(ctx, storj.NodeID{}, 1, []storj.NodeID{}, nil)
		require.NoError(t, err)
		require.Len(t, selected, 1)
		require.NotEqual(t, testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID, selected[0].ID)
		require.NotEqual(t, testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID, selected[0].ID)
	}

}

func TestTargetType(t *testing.T) {
	r := targetType(float64(1), ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		return 0
	}))
	require.Equal(t, "ScoreNode", r.Name())
}

func TestCompare(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// Create two score nodes for testing
	scoreNode1 := ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		return float64(node.FreeDisk)
	})

	scoreNode2 := ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		return float64(node.PieceCount)
	})

	// Create nodes with different values
	node1 := &SelectedNode{FreeDisk: 10, PieceCount: 5}
	node2 := &SelectedNode{FreeDisk: 10, PieceCount: 3}
	node3 := &SelectedNode{FreeDisk: 5, PieceCount: 10}

	compareFunc := Compare(scoreNode1, scoreNode2)
	compareFn := compareFunc(storj.NodeID{})

	// Test cases
	require.Equal(t, 0, compareFn(node1, node1), "Node should be equal to itself")
	require.Equal(t, 1, compareFn(node1, node2), "Node1 and Node2 have same FreeDisk, but Node1 has higher PieceCount")
	require.Equal(t, -1, compareFn(node2, node1), "Node2 and Node1 have same FreeDisk, but Node2 has lower PieceCount")
	require.Equal(t, 1, compareFn(node1, node3), "Node1 has higher FreeDisk than Node3, so first score decides")
	require.Equal(t, -1, compareFn(node3, node1), "Node3 has lower FreeDisk than Node1, so first score decides")

	// Test with NaN values
	nodeNaN1 := &SelectedNode{FreeDisk: 0, PieceCount: 0} // Will return NaN for both scores
	nodeNaN2 := &SelectedNode{FreeDisk: 5, PieceCount: 0} // Will return NaN for second score

	scoreNodeNaN1 := ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		if node.FreeDisk == 0 {
			return math.NaN()
		}
		return float64(node.FreeDisk)
	})

	scoreNodeNaN2 := ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		if node.PieceCount == 0 {
			return math.NaN()
		}
		return float64(node.PieceCount)
	})

	compareNaNFunc := Compare(scoreNodeNaN1, scoreNodeNaN2)
	compareNaNFn := compareNaNFunc(storj.NodeID{})

	require.Equal(t, 0, compareNaNFn(nodeNaN1, nodeNaN1), "Both nodes have NaN for both scores")
	require.Equal(t, 1, compareNaNFn(nodeNaN1, nodeNaN2), "First node has NaN for first score, second has value")
	require.Equal(t, -1, compareNaNFn(nodeNaN2, nodeNaN1), "Second node has NaN for first score, first has value")
}

func TestArithmetic(t *testing.T) {
	zeroSigner, err := storj.NodeIDFromString("1111111111111111111111111111111VyS547o")
	require.NoError(t, err)
	node := SelectedNode{
		FreeDisk: 2,
		Tags: NodeTags{
			{Name: "weight", Value: []byte("3"), Signer: zeroSigner},
		},
	}

	env := map[any]any{}
	env["node_value"] = func(name string) NodeValue {
		val, err := CreateNodeValue(name)
		if err != nil {
			panic("Invalid node value: " + err.Error())
		}
		return val
	}
	env["uploadSuccessTracker"] = ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
		return 1.0
	})
	AddArithmetic(env)

	t.Run("add", func(t *testing.T) {
		res, err := mito.Eval("2 + 3", env)
		require.NoError(t, err)
		require.Equal(t, 5, res)
	})

	t.Run("subtract", func(t *testing.T) {
		res, err := mito.Eval("2 - 3", env)
		require.NoError(t, err)
		require.Equal(t, -1, res)
	})

	t.Run("multiply", func(t *testing.T) {
		res, err := mito.Eval("2 * -1.0", env)
		require.NoError(t, err)
		require.Equal(t, -2.0, res)
	})

	t.Run("multiply with precedence", func(t *testing.T) {
		res, err := mito.Eval("(2 + 10) * -1.0", env)
		require.NoError(t, err)
		require.Equal(t, -12.0, res)
	})

	t.Run("divide", func(t *testing.T) {
		res, err := mito.Eval("node_value(\"free_disk\") / 2.0", env)
		require.NoError(t, err)
		require.Equal(t, 1.0, res.(NodeValue)(node))
	})

	t.Run("min", func(t *testing.T) {
		res, err := mito.Eval("min(node_value(\"free_disk\"),1.0)", env)
		require.NoError(t, err)
		require.Equal(t, 1.0, res.(NodeValue)(node))
	})

	t.Run("max", func(t *testing.T) {
		res, err := mito.Eval("max(node_value(\"free_disk\"),1.0)", env)
		require.NoError(t, err)
		require.Equal(t, 2.0, res.(NodeValue)(node))
	})

	t.Run("pow", func(t *testing.T) {
		t.Run("integers", func(t *testing.T) {
			res, err := mito.Eval("2 ^ 3", env)
			require.NoError(t, err)
			require.Equal(t, 8.0, res)
		})

		t.Run("integer and float", func(t *testing.T) {
			res, err := mito.Eval("2 ^ 3.0", env)
			require.NoError(t, err)
			require.Equal(t, 8.0, res)
		})

		t.Run("node_value and float", func(t *testing.T) {
			res, err := mito.Eval("node_value(\"free_disk\") ^ 3.0", env)
			require.NoError(t, err)
			i := res.(NodeValue)(node)
			require.Equal(t, 8.0, i)
		})

		t.Run("node field and tag", func(t *testing.T) {
			res, err := mito.Eval("node_value(\"free_disk\") ^ node_value(\"tag:1111111111111111111111111111111VyS547o/weight\")", env)
			require.NoError(t, err)
			i := res.(NodeValue)(node)
			require.Equal(t, 8.0, i)
		})

		t.Run("score node and node field", func(t *testing.T) {
			res, err := mito.Eval("uploadSuccessTracker + node_value(\"free_disk\") ^ 2", env)
			require.NoError(t, err)
			i := res.(ScoreNode).Get(storj.NodeID{})(&node)
			require.Equal(t, 5.0, i)
		})
	})

	t.Run("round", func(t *testing.T) {
		t.Run("integer", func(t *testing.T) {
			res, err := mito.Eval("round(5)", env)
			require.NoError(t, err)
			require.Equal(t, int64(5), res)
		})

		t.Run("float", func(t *testing.T) {
			res, err := mito.Eval("round(5.4)", env)
			require.NoError(t, err)
			require.Equal(t, 5.0, res)
		})

		t.Run("float rounding up", func(t *testing.T) {
			res, err := mito.Eval("round(5.6)", env)
			require.NoError(t, err)
			require.Equal(t, 6.0, res)
		})

		t.Run("negative float rounding", func(t *testing.T) {
			res, err := mito.Eval("round(-5.6)", env)
			require.NoError(t, err)
			require.Equal(t, -6.0, res)
		})

		t.Run("node_value", func(t *testing.T) {
			nodeWithFloat := SelectedNode{
				FreeDisk: 2,
			}

			res, err := mito.Eval("round(node_value(\"free_disk\"))", env)
			require.NoError(t, err)
			i := res.(NodeValue)(nodeWithFloat)
			require.Equal(t, 2.0, i)
		})

		t.Run("expression result", func(t *testing.T) {
			res, err := mito.Eval("round(2.5 + 0.1)", env)
			require.NoError(t, err)
			require.Equal(t, 3.0, res)
		})

		t.Run("score node", func(t *testing.T) {
			scoreNodeFloat := ScoreNodeFunc(func(uplink storj.NodeID, node *SelectedNode) float64 {
				return 4.7
			})
			env["floatScoreTracker"] = scoreNodeFloat

			res, err := mito.Eval("round(floatScoreTracker)", env)
			require.NoError(t, err)
			i := res.(ScoreNode).Get(storj.NodeID{})(&node)
			require.Equal(t, 5.0, i)
		})
	})
}

type mockTracker struct {
}

func (m mockTracker) Get(uplink storj.NodeID) func(node *SelectedNode) float64 {
	return func(node *SelectedNode) float64 { return 0 }
}
