// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/nodeselection"
)

func TestPlacementFromString(t *testing.T) {
	signer, err := storj.NodeIDFromString("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4")
	require.NoError(t, err)

	t.Run("single country", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`11:country("GB")`)
		require.NoError(t, err)
		filters := p.placements[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.False(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))
	})

	t.Run("tag rule", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`11:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar")`)
		require.NoError(t, err)
		filters := p.placements[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.MatchInclude(&nodeselection.SelectedNode{
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))
		require.False(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))
	})

	t.Run("all rules", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`11:all(country("GB"),tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar"))`)
		require.NoError(t, err)
		filters := p.placements[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))
		require.False(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.False(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))
	})

	t.Run("multi rule", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`11:country("GB");12:country("DE")`)
		require.NoError(t, err)

		filters := p.placements[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.False(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))

		filters = p.placements[storj.PlacementConstraint(12)]
		require.NotNil(t, filters)
		require.False(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.True(t, filters.MatchInclude(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))

	})

}
