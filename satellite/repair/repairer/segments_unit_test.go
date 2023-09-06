// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

func TestClassify(t *testing.T) {
	ctx := testcontext.New(t)

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

		c := &overlay.ConfigurablePlacementRule{}
		require.NoError(t, c.Set(""))
		s := SegmentRepairer{
			placementRules: overlay.NewPlacementDefinitions().CreateFilters,
		}
		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4)
		result, err := s.classifySegmentPiecesWithNodes(ctx, metabase.Segment{Pieces: pieces}, allNodeIDs(pieces), selectedNodes)
		require.NoError(t, err)

		require.Equal(t, 0, len(result.MissingPiecesSet))
		require.Equal(t, 0, len(result.ClumpedPiecesSet))
		require.Equal(t, 0, len(result.OutOfPlacementPiecesSet))
		require.Equal(t, 0, result.NumUnhealthyRetrievable)
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

		c, err := overlay.ConfigurablePlacementRule{
			PlacementRules: `10:country("GB")`,
		}.Parse()
		require.NoError(t, err)

		s := SegmentRepairer{
			placementRules:   c.CreateFilters,
			doPlacementCheck: true,
			log:              zaptest.NewLogger(t),
		}

		pieces := createPieces(selectedNodes, 1, 2, 3, 4, 7, 8)
		result, err := s.classifySegmentPiecesWithNodes(ctx, metabase.Segment{Pieces: pieces, Placement: 10}, allNodeIDs(pieces), getNodes(selectedNodes, pieces))
		require.NoError(t, err)

		require.Equal(t, 0, len(result.MissingPiecesSet))
		require.Equal(t, 0, len(result.ClumpedPiecesSet))
		// 1,2,3 are in Germany instead of GB
		require.Equal(t, 3, len(result.OutOfPlacementPiecesSet))
		require.Equal(t, 3, result.NumUnhealthyRetrievable)
	})

	t.Run("out of placement and offline", func(t *testing.T) {
		// all nodes are in wrong region and half of them are offline
		var selectedNodes = generateNodes(10, func(ix int) bool {
			return ix < 5
		}, func(ix int, node *nodeselection.SelectedNode) {
			node.CountryCode = location.Germany
		})

		c, err := overlay.ConfigurablePlacementRule{
			PlacementRules: `10:country("GB")`,
		}.Parse()
		require.NoError(t, err)

		s := SegmentRepairer{
			placementRules:   c.CreateFilters,
			doPlacementCheck: true,
		}

		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		result, err := s.classifySegmentPiecesWithNodes(ctx, metabase.Segment{Pieces: pieces, Placement: 10}, allNodeIDs(pieces), getNodes(selectedNodes, pieces))
		require.NoError(t, err)

		// offline nodes
		require.Equal(t, 5, len(result.MissingPiecesSet))
		require.Equal(t, 0, len(result.ClumpedPiecesSet))
		require.Equal(t, 10, len(result.OutOfPlacementPiecesSet))
		require.Equal(t, 5, result.NumUnhealthyRetrievable)
		numHealthy := len(pieces) - len(result.MissingPiecesSet) - result.NumUnhealthyRetrievable
		require.Equal(t, 0, numHealthy)

	})

	t.Run("normal declumping (subnet check)", func(t *testing.T) {
		var selectedNodes = generateNodes(10, func(ix int) bool {
			return ix < 5
		}, func(ix int, node *nodeselection.SelectedNode) {
			node.LastNet = fmt.Sprintf("127.0.%d.0", ix/2)
		})

		c := overlay.NewPlacementDefinitions()
		s := SegmentRepairer{
			placementRules: c.CreateFilters,
			doDeclumping:   true,
			log:            zaptest.NewLogger(t),
		}

		// first 5: online, 2 in each subnet --> healthy: one from (0,1) (2,3) (4), offline: (5,6) but 5 is in the same subnet as 6
		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4, 5, 6)
		result, err := s.classifySegmentPiecesWithNodes(ctx, metabase.Segment{Pieces: pieces}, allNodeIDs(pieces), getNodes(selectedNodes, pieces))
		require.NoError(t, err)

		// offline nodes
		require.Equal(t, 2, len(result.MissingPiecesSet))
		require.Equal(t, 3, len(result.ClumpedPiecesSet))
		require.Equal(t, 0, len(result.OutOfPlacementPiecesSet))
		require.Equal(t, 2, result.NumUnhealthyRetrievable)
		numHealthy := len(pieces) - len(result.MissingPiecesSet) - result.NumUnhealthyRetrievable
		require.Equal(t, 3, numHealthy)

	})

	t.Run("declumping but with no subnet filter", func(t *testing.T) {
		var selectedNodes = generateNodes(10, func(ix int) bool {
			return ix < 5
		}, func(ix int, node *nodeselection.SelectedNode) {
			node.LastNet = fmt.Sprintf("127.0.%d.0", ix/2)
			node.CountryCode = location.UnitedKingdom
		})

		c, err := overlay.ConfigurablePlacementRule{
			PlacementRules: fmt.Sprintf(`10:annotated(country("GB"),annotation("%s","%s"))`, nodeselection.AutoExcludeSubnet, nodeselection.AutoExcludeSubnetOFF),
		}.Parse()
		require.NoError(t, err)

		s := SegmentRepairer{
			placementRules: c.CreateFilters,
			doDeclumping:   true,
		}

		// first 5: online, 2 in each subnet --> healthy: one from (0,1) (2,3) (4), offline: (5,6) but 5 is in the same subnet as 6
		pieces := createPieces(selectedNodes, 0, 1, 2, 3, 4, 5, 6)
		result, err := s.classifySegmentPiecesWithNodes(ctx, metabase.Segment{Pieces: pieces, Placement: 10}, allNodeIDs(pieces), getNodes(selectedNodes, pieces))
		require.NoError(t, err)

		// offline nodes
		require.Equal(t, 2, len(result.MissingPiecesSet))
		require.Equal(t, 0, len(result.ClumpedPiecesSet))
		require.Equal(t, 0, len(result.OutOfPlacementPiecesSet))
		require.Equal(t, 0, result.NumUnhealthyRetrievable)
		numHealthy := len(pieces) - len(result.MissingPiecesSet) - result.NumUnhealthyRetrievable
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

func allNodeIDs(pieces metabase.Pieces) (res []storj.NodeID) {
	for _, piece := range pieces {
		res = append(res, piece.StorageNode)
	}
	return res
}
