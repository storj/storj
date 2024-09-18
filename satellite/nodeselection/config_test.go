// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/location"
)

func TestParsedConfig(t *testing.T) {

	config, err := LoadConfig("config_test.yaml", NewPlacementConfigEnvironment(mockTracker{}))
	require.NoError(t, err)
	require.Len(t, config, 7)

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
		selected, err := config[0].Selector([]*SelectedNode{
			{
				Vetted: false,
			},
		}, nil)(storj.NodeID{}, 1, nil, nil)

		// having: new, requires: 0% unvetted = 100% vetted
		require.Len(t, selected, 0)
		require.NoError(t, err)
	}

	{
		// smoketest for creating choice of two selector
		selected, err := config[2].Selector(
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
		)(storj.NodeID{}, 1, nil, nil)

		require.Len(t, selected, 1)
		require.NoError(t, err)
	}
}

func TestParsedConfigWithoutTracker(t *testing.T) {
	// tracker is not available for certain microservices (like repair). Still the placement should work.
	config, err := LoadConfig("config_test.yaml", NewPlacementConfigEnvironment(nil))
	require.NoError(t, err)
	require.Len(t, config, 7)

	// smoketest for creating choice of two selector
	selected, err := config[2].Selector(
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
	)(storj.NodeID{}, 1, nil, nil)

	require.Len(t, selected, 1)
	require.NoError(t, err)

}

func TestFilterFromString(t *testing.T) {
	filter, err := FilterFromString(`exclude(nodelist("filter_testdata.txt"))`)
	require.NoError(t, err)

	require.False(t, filter.Match(&SelectedNode{
		ID: testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID,
	}))
	require.True(t, filter.Match(&SelectedNode{
		ID: testidentity.MustPregeneratedIdentity(3, storj.LatestIDVersion()).ID,
	}))

}

func TestSelectorFromString(t *testing.T) {
	selector, err := SelectorFromString(`filter(exclude(nodelist("filter_testdata.txt")),random())`, nil)
	require.NoError(t, err)

	// initialize the node space
	var nodes []*SelectedNode
	for i := 0; i < 10; i++ {
		nodes = append(nodes, &SelectedNode{
			ID: testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
		})
	}

	initialized := selector(nodes, nil)

	for i := 0; i < 100; i++ {
		selected, err := initialized(storj.NodeID{}, 1, []storj.NodeID{}, nil)
		require.NoError(t, err)
		require.Len(t, selected, 1)
		require.NotEqual(t, testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID, selected[0].ID)
		require.NotEqual(t, testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID, selected[0].ID)
	}

}

type mockTracker struct {
}

func (m mockTracker) Get(uplink storj.NodeID) func(node *SelectedNode) float64 {
	return func(node *SelectedNode) float64 { return 0 }
}
