// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
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
