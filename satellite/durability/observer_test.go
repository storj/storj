// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package durability

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/shared/location"
)

type nodeList struct {
	nodes []*nodeselection.SelectedNode
}

func (n nodeList) GetNodes(ctx context.Context, validUpTo time.Time, nodeIDs []storj.NodeID, selectedNodes []nodeselection.SelectedNode) ([]nodeselection.SelectedNode, error) {
	var res []nodeselection.SelectedNode
	for _, nodeID := range nodeIDs {
		for _, node := range n.nodes {
			if node.ID == nodeID {
				res = append(res, *node)
			}
		}
	}
	return res, nil
}

var _ NodeGetter = (*nodeList)(nil)

func TestDurability(t *testing.T) {
	var storageNodes []*nodeselection.SelectedNode
	var aliases []metabase.NodeAliasEntry
	for i := 0; i < 10; i++ {
		node := &nodeselection.SelectedNode{
			ID:      testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
			LastNet: fmt.Sprintf("127.0.%d.0", i%3),
			Online:  true,
		}
		storageNodes = append(storageNodes, node)
		aliases = append(aliases, metabase.NodeAliasEntry{
			ID:    node.ID,
			Alias: metabase.NodeAlias(i),
		})
	}

	segment := func(nodes []*nodeselection.SelectedNode, ix ...int) (res rangedloop.Segment) {
		var aliasPieces metabase.AliasPieces
		var pieces []metabase.Piece
		for n, i := range ix {
			aliasPieces = append(aliasPieces, metabase.AliasPiece{
				Number: uint16(n),
				Alias:  metabase.NodeAlias(i),
			})
			pieces = append(pieces, metabase.Piece{
				Number:      uint16(n),
				StorageNode: nodes[i].ID,
			})
		}

		res.StreamID = testrand.UUID()
		res.Position = metabase.SegmentPosition{
			Part:  0,
			Index: 0,
		}

		// it's not inline if non-default redundancy is set.
		res.Redundancy = storj.RedundancyScheme{
			RequiredShares: 3,
			ShareSize:      123,
		}

		res.AliasPieces = aliasPieces
		res.Pieces = pieces
		res.RootPieceID = testrand.PieceID()

		return res
	}

	ctx := testcontext.New(t)
	c := NewDurability(nil, nil, nodeList{nodes: storageNodes}, "net", func(node *nodeselection.SelectedNode) string {
		return node.LastNet
	}, 0)

	aliasMap := metabase.NewNodeAliasMap(aliases)
	for _, node := range storageNodes {
		c.nodes = append(c.nodes, *node)
	}

	c.classifyNodeAliases(aliasMap)

	fork, err := c.Fork(ctx)
	require.NoError(t, err)

	{
		special := segment(storageNodes, 3, 6, 9, 1)
		special.Placement = storj.PlacementConstraint(5)

		// first batch
		err = fork.Process(ctx, []rangedloop.Segment{
			special,
			segment(storageNodes, 3, 6, 9, 1),
		})
		require.NoError(t, err)

		// second batch
		err = fork.Process(ctx, []rangedloop.Segment{
			segment(storageNodes, 2, 3, 4, 7),
			segment(storageNodes, 1, 2, 3, 4, 6, 7, 8),
		})
		require.NoError(t, err)

		err = c.Join(ctx, fork)
		require.NoError(t, err)
	}

	// without removing biggest provider
	require.Equal(t, 0, c.healthStat[0][0].Buckets[0].SegmentCount)
	require.Equal(t, 2, c.healthStat[0][0].Buckets[1].SegmentCount)
	require.Equal(t, 0, c.healthStat[0][0].Buckets[2].SegmentCount)
	require.Equal(t, 0, c.healthStat[0][0].Buckets[3].SegmentCount)
	require.Equal(t, 1, c.healthStat[0][0].Buckets[4].SegmentCount)

	// with removing biggest provider
	require.Equal(t, 1, c.healthStat[1][0].NegativeBuckets[1].SegmentCount)
	require.Equal(t, 1, c.healthStat[1][0].NegativeBuckets[2].SegmentCount)
	require.Equal(t, 0, c.healthStat[1][0].Buckets[0].SegmentCount)
	require.Equal(t, 1, c.healthStat[1][0].Buckets[1].SegmentCount)

	// placement 5
	require.Equal(t, 0, c.healthStat[0][5].Buckets[0].SegmentCount)
	require.Equal(t, 1, c.healthStat[0][5].Buckets[1].SegmentCount)

	require.Equal(t, 1, c.healthMatrix.Find(5, 1, 1))
	require.Equal(t, 2, c.healthMatrix.Find(0, 1, 1))
	require.Equal(t, 1, c.healthMatrix.Find(0, 4, 4))
	// usually called with c.Start()
	c.resetStat()

	fork, err = c.Fork(ctx)
	require.NoError(t, err)
	err = c.Join(ctx, fork)
	require.NoError(t, err)
	require.Equal(t, 0, c.healthStat[0][0].Buckets[1].SegmentCount)

}

func BenchmarkDurabilityProcess(b *testing.B) {
	ctx := b.Context()

	rng := rand.New(rand.NewSource(0))

	nodeNo := 20000
	// create 2500 segments (usual observer loop batch size) with 80 pieces
	segmentNo := 2500
	pieceNo := 80
	if testing.Short() {
		nodeNo = 10
		segmentNo = 10
		pieceNo = 10
	}

	nodeMap := make(map[storj.NodeID]*nodeselection.SelectedNode)
	var aliasToNode []*nodeselection.SelectedNode
	{
		// generating nodes and node aliases.
		for i := 0; i < nodeNo; i++ {
			id := testrand.NodeID()
			node := &nodeselection.SelectedNode{
				ID:          id,
				LastNet:     "10.8.0.0",
				CountryCode: location.UnitedStates,
				Email:       fmt.Sprintf("test+%d@asd.hu", i%2),
			}
			nodeMap[node.ID] = node
			aliasToNode = append(aliasToNode, node)

		}
	}

	var segments []rangedloop.Segment
	{

		for i := 0; i < segmentNo; i++ {
			var id uuid.UUID
			rng.Read(id[:])

			var pieces metabase.Pieces
			var aliasPieces metabase.AliasPieces
			for j := 0; j < pieceNo; j++ {
				nodeIx := rand.Intn(len(aliasToNode) - 1)
				pieces = append(pieces, metabase.Piece{
					Number:      uint16(j),
					StorageNode: aliasToNode[nodeIx].ID,
				})
				aliasPieces = append(aliasPieces, metabase.AliasPiece{
					Number: uint16(j),
					Alias:  metabase.NodeAlias(nodeIx),
				})
			}
			segments = append(segments, rangedloop.Segment{
				StreamID: id,
				Position: metabase.SegmentPosition{
					Part:  1,
					Index: 1,
				},
				CreatedAt:   time.Now(),
				Pieces:      pieces,
				AliasPieces: aliasPieces,
			})
		}
	}

	d := ObserverFork{
		nodesCache: nodeList{nodes: aliasToNode},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkProcess(ctx, b, d, segments)
	}
}

func benchmarkProcess(ctx context.Context, b *testing.B, d ObserverFork, segments []rangedloop.Segment) {
	err := d.Process(ctx, segments)
	require.NoError(b, err)
}
