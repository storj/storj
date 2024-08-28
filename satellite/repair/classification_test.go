// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/location"
)

func TestClassifySegmentPieces(t *testing.T) {
	getNodes := func(nodes []nodeselection.SelectedNode, pieces metabase.Pieces) (res []nodeselection.SelectedNode) {
		for _, piece := range pieces {
			for _, node := range nodes {
				if node.ID == piece.StorageNode {
					res = append(res, node)
					break
				}
			}

		}
		return res
	}
	t.Run("all online", func(t *testing.T) {
		var selectedNodes = generateNodes(5, func(ix int) bool {
			return true
		}, func(ix int, node *nodeselection.SelectedNode) {

		})

		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4)
		result := ClassifySegmentPieces(pieces, getNodes(selectedNodes, pieces), map[location.CountryCode]struct{}{}, true, false, nodeselection.TestPlacementDefinitions()[0])

		require.Equal(t, 0, result.Missing.Count())
		require.Equal(t, 0, result.Clumped.Count())
		require.Equal(t, 0, result.OutOfPlacement.Count())
		require.Equal(t, 0, result.UnhealthyRetrievable.Count())
	})

	t.Run("out of placement", func(t *testing.T) {
		var selectedNodes = generateNodes(10, func(ix int) bool {
			return true
		}, func(ix int, node *nodeselection.SelectedNode) {
			if ix < 4 {
				node.CountryCode = location.Germany
			} else {
				node.CountryCode = location.UnitedKingdom
			}

		})

		c, err := nodeselection.ConfigurablePlacementRule{
			PlacementRules: `10:country("GB")`,
		}.Parse(nil, nil)
		require.NoError(t, err)

		pieces := createPieces(selectedNodes, 1, 2, 3, 4, 7, 8)
		result := ClassifySegmentPieces(pieces, getNodes(selectedNodes, pieces), map[location.CountryCode]struct{}{}, true, false, c[10])

		require.Equal(t, 0, result.Missing.Count())
		require.Equal(t, 0, result.Clumped.Count())
		// 1,2,3 are in Germany instead of GB
		require.Equal(t, 3, result.OutOfPlacement.Count())
		require.Equal(t, 3, result.UnhealthyRetrievable.Count())
	})

	t.Run("out of placement and offline", func(t *testing.T) {
		// all nodes are in wrong region and half of them are offline
		var selectedNodes = generateNodes(10, func(ix int) bool {
			return ix < 5
		}, func(ix int, node *nodeselection.SelectedNode) {
			node.CountryCode = location.Germany
		})

		c, err := nodeselection.ConfigurablePlacementRule{
			PlacementRules: `10:country("GB")`,
		}.Parse(nil, nil)
		require.NoError(t, err)

		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		result := ClassifySegmentPieces(pieces, getNodes(selectedNodes, pieces), map[location.CountryCode]struct{}{}, true, false, c[10])

		// offline nodes
		require.Equal(t, 5, result.Missing.Count())
		require.Equal(t, 0, result.Clumped.Count())
		require.Equal(t, 10, result.OutOfPlacement.Count())
		require.Equal(t, 5, result.UnhealthyRetrievable.Count())
		numHealthy := len(pieces) - result.Missing.Count() - result.UnhealthyRetrievable.Count()
		require.Equal(t, 0, numHealthy)

	})

	t.Run("normal declumping (subnet check)", func(t *testing.T) {
		var selectedNodes = generateNodes(10, func(ix int) bool {
			return ix < 5
		}, func(ix int, node *nodeselection.SelectedNode) {
			node.LastNet = fmt.Sprintf("127.0.%d.0", ix/2)
		})

		c := nodeselection.TestPlacementDefinitions()

		// first 5: online, 2 in each subnet --> healthy: one from (0,1) (2,3) (4), offline: (5,6) but 5 is in the same subnet as 6
		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4, 5, 6)
		result := ClassifySegmentPieces(pieces, getNodes(selectedNodes, pieces), map[location.CountryCode]struct{}{}, true, true, c[0])

		// offline nodes
		require.Equal(t, 2, result.Missing.Count())
		require.Equal(t, 3, result.Clumped.Count())
		require.Equal(t, 0, result.OutOfPlacement.Count())
		require.Equal(t, 2, result.UnhealthyRetrievable.Count())
		numHealthy := len(pieces) - result.Missing.Count() - result.UnhealthyRetrievable.Count()
		require.Equal(t, 3, numHealthy)

	})

	t.Run("declumping but with no subnet filter", func(t *testing.T) {
		var selectedNodes = generateNodes(10, func(ix int) bool {
			return ix < 5
		}, func(ix int, node *nodeselection.SelectedNode) {
			node.LastNet = fmt.Sprintf("127.0.%d.0", ix/2)
			node.CountryCode = location.UnitedKingdom
		})

		c, err := nodeselection.ConfigurablePlacementRule{
			PlacementRules: fmt.Sprintf(`10:annotated(country("GB"),annotation("%s","%s"))`, nodeselection.AutoExcludeSubnet, nodeselection.AutoExcludeSubnetOFF),
		}.Parse(nil, nil)
		require.NoError(t, err)

		// first 5: online, 2 in each subnet --> healthy: one from (0,1) (2,3) (4), offline: (5,6) but 5 is in the same subnet as 6
		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4, 5, 6)
		result := ClassifySegmentPieces(pieces, getNodes(selectedNodes, pieces), map[location.CountryCode]struct{}{}, true, true, c[10])

		// offline nodes
		require.Equal(t, 2, result.Missing.Count())
		require.Equal(t, 0, result.Clumped.Count())
		require.Equal(t, 0, result.OutOfPlacement.Count())
		require.Equal(t, 0, result.UnhealthyRetrievable.Count())
		numHealthy := len(pieces) - result.Missing.Count() - result.UnhealthyRetrievable.Count()
		require.Equal(t, 5, numHealthy)

	})

}

func generateNodes(num int, isOnline func(i int) bool, config func(ix int, node *nodeselection.SelectedNode)) (selectedNodes []nodeselection.SelectedNode) {
	for i := 0; i < num; i++ {
		node := nodeselection.SelectedNode{
			ID:     testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
			Online: isOnline(i),
		}
		config(i, &node)
		selectedNodes = append(selectedNodes, node)
	}
	return
}

func createPieces(selectedNodes []nodeselection.SelectedNode, indexes ...int) (res metabase.Pieces) {
	for _, index := range indexes {
		piece := metabase.Piece{
			Number: uint16(index),
		}
		piece.StorageNode = selectedNodes[index].ID
		res = append(res, piece)
	}
	return
}
