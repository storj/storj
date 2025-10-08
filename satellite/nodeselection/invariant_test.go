// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/storj/private/intset"
	"storj.io/storj/satellite/metabase"
)

func TestClumpingByAnyTag(t *testing.T) {
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

	invariant := ClumpingByAnyTag("owner", 2)
	result := invariant(
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
	require.True(t, result.Contains(11))
}

func TestInvariantFilter(t *testing.T) {
	signer := testidentity.MustPregeneratedSignedIdentity(100, storj.LatestIDVersion())

	node := func(ix int, placement int) SelectedNode {
		return SelectedNode{
			ID: testidentity.MustPregeneratedSignedIdentity(ix, storj.LatestIDVersion()).ID,
			Tags: NodeTags{
				{
					Name:   "placement",
					Value:  []byte(strconv.Itoa(placement)),
					Signer: signer.ID,
				},
			},
		}
	}
	piece := func(ix int, nodeIx int) metabase.Piece {
		return metabase.Piece{
			Number: uint16(ix), StorageNode: testidentity.MustPregeneratedSignedIdentity(nodeIx, storj.LatestIDVersion()).ID,
		}

	}

	nf, err := FilterFromString(fmt.Sprintf(`tag("%s","placement","2")`, signer.ID), NewPlacementConfigEnvironment(nil, nil))
	require.NoError(t, err)

	invariant := FilterInvariant(nf)
	result := invariant(
		metabase.Pieces{
			piece(1, 1),
			piece(3, 2),
			piece(5, 3),
			piece(9, 4),
			piece(10, 5),
			piece(11, 6),
		},
		[]SelectedNode{
			node(1, 2),
			node(2, 2),
			node(3, 2),
			node(4, 2),
			node(5, 3),
			node(6, 3),
		})

	// last two pieces are placement=3 --> out-of-placement
	require.Equal(t, 2, result.Count())
	require.True(t, result.Contains(10))
	require.True(t, result.Contains(11))
}

func TestCombinedInvariantFilter(t *testing.T) {
	i1 := func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
		is := intset.NewSet(5)
		is.Include(1)
		is.Include(3)
		return is
	}

	i2 := func(pieces metabase.Pieces, nodes []SelectedNode) intset.Set {
		is := intset.NewSet(5)
		is.Include(1)
		is.Include(2)
		return is
	}

	node := func(ix int) SelectedNode {
		return SelectedNode{
			ID: testidentity.MustPregeneratedSignedIdentity(ix, storj.LatestIDVersion()).ID,
		}
	}

	piece := func(ix int, nodeIx int) metabase.Piece {
		return metabase.Piece{
			Number: uint16(ix), StorageNode: testidentity.MustPregeneratedSignedIdentity(nodeIx, storj.LatestIDVersion()).ID,
		}

	}

	c := CombinedInvariant(i1, i2)
	result := c(
		metabase.Pieces{
			piece(1, 1),
			piece(3, 2),
			piece(5, 3),
			piece(9, 4),
			piece(10, 5),
			piece(11, 6),
		},
		[]SelectedNode{
			node(1),
			node(2),
			node(3),
			node(4),
			node(5),
			node(6),
		})

	//	require.Equal(t, 3, result.Count())
	require.True(t, result.Contains(1))
	require.True(t, result.Contains(2))
	require.True(t, result.Contains(3))

}
