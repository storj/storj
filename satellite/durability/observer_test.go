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
	"storj.io/common/storj/location"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
)

func TestDurability(t *testing.T) {
	var storageNodes []*nodeselection.SelectedNode
	var aliases []metabase.NodeAliasEntry
	for i := 0; i < 10; i++ {
		node := &nodeselection.SelectedNode{
			ID:      testidentity.MustPregeneratedIdentity(i, storj.LatestIDVersion()).ID,
			LastNet: fmt.Sprintf("127.0.%d.0", i%3),
		}
		storageNodes = append(storageNodes, node)
		aliases = append(aliases, metabase.NodeAliasEntry{
			ID:    node.ID,
			Alias: metabase.NodeAlias(i),
		})
	}

	ctx := testcontext.New(t)
	c := NewDurability(nil, nil, []NodeClassifier{
		func(node *nodeselection.SelectedNode) string {
			return "net:" + node.LastNet
		}}, 0, 0)

	c.aliasMap = metabase.NewNodeAliasMap(aliases)
	for _, node := range storageNodes {
		c.nodes[node.ID] = node
	}

	fork, err := c.Fork(ctx)
	require.NoError(t, err)

	{
		// first batch
		err = fork.Process(ctx, []rangedloop.Segment{
			{
				StreamID: testrand.UUID(),
				Position: metabase.SegmentPosition{
					Part:  1,
					Index: 1,
				},
				AliasPieces: pieces(storageNodes, 3, 6, 9, 1),
			},
		})
		require.NoError(t, err)

		err = c.Join(ctx, fork)
		require.NoError(t, err)
	}

	{
		// second batch
		err = fork.Process(ctx, []rangedloop.Segment{
			{
				StreamID: testrand.UUID(),
				Position: metabase.SegmentPosition{
					Part:  1,
					Index: 1,
				},
				AliasPieces: pieces(storageNodes, 2, 3, 4, 7),
			},
			{
				StreamID: testrand.UUID(),
				Position: metabase.SegmentPosition{
					Part:  1,
					Index: 1,
				},
				AliasPieces: pieces(storageNodes, 1, 2, 3, 4, 6, 7, 8),
			},
		})
		require.NoError(t, err)

		err = c.Join(ctx, fork)
		require.NoError(t, err)
	}

	require.Equal(t, 1, c.healthStat["net:127.0.0.0"].Min())
	require.Equal(t, 2, c.healthStat["net:127.0.1.0"].Min())
	require.Equal(t, 3, c.healthStat["net:127.0.2.0"].Min())
}

func TestDurabilityUnknownNode(t *testing.T) {
	var storageNodes []*nodeselection.SelectedNode
	var aliases []metabase.NodeAliasEntry

	node := &nodeselection.SelectedNode{
		ID:      testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion()).ID,
		LastNet: "127.0.0.1",
	}
	storageNodes = append(storageNodes, node)
	aliases = append(aliases, metabase.NodeAliasEntry{
		ID:    node.ID,
		Alias: metabase.NodeAlias(0),
	})

	ctx := testcontext.New(t)
	c := NewDurability(nil, nil, []NodeClassifier{
		func(node *nodeselection.SelectedNode) string {
			return "net:" + node.LastNet
		}}, 0, 0)

	c.aliasMap = metabase.NewNodeAliasMap(aliases)
	for _, node := range storageNodes {
		c.nodes[node.ID] = node
	}

	fork, err := c.Fork(ctx)
	require.NoError(t, err)

	// note: second piece points to an alias which was not preloaded (newly inserted).
	err = fork.Process(ctx, []rangedloop.Segment{
		{
			StreamID: testrand.UUID(),
			Position: metabase.SegmentPosition{
				Part:  1,
				Index: 1,
			},
			AliasPieces: metabase.AliasPieces{
				metabase.AliasPiece{
					Number: 1,
					Alias:  0,
				},
				metabase.AliasPiece{
					Number: 2,
					Alias:  9999,
				},
			},
		},
	})
	require.NoError(t, err)

	err = c.Join(ctx, fork)
	require.NoError(t, err)
	// note: the newly created node (alias 9999) is not considered.
	require.Equal(t, 0, c.healthStat["net:127.0.0.1"].Min())
}

func pieces(nodes []*nodeselection.SelectedNode, ix ...int) (res metabase.AliasPieces) {
	for n, i := range ix {
		res = append(res, metabase.AliasPiece{
			Number: uint16(n),
			Alias:  metabase.NodeAlias(i),
		})
	}
	return res
}

func BenchmarkDurabilityProcess(b *testing.B) {
	ctx := context.TODO()

	rng := rand.New(rand.NewSource(0))

	nodeNo := 20000
	if testing.Short() {
		nodeNo = 10
	}

	nodeMap := make(map[storj.NodeID]*nodeselection.SelectedNode)
	var aliasToNode []*nodeselection.SelectedNode
	var nodeAliases []metabase.NodeAliasEntry
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
			nodeAliases = append(nodeAliases, metabase.NodeAliasEntry{
				ID:    node.ID,
				Alias: metabase.NodeAlias(i),
			})

		}
	}
	aliases := metabase.NewNodeAliasMap(nodeAliases)

	var segments []rangedloop.Segment
	{
		// create 2500 segments (usual observer loop batch size) with 80 pieces
		for i := 0; i < 2500; i++ {
			var id uuid.UUID
			rng.Read(id[:])

			var pieces metabase.Pieces
			var aliasPieces metabase.AliasPieces
			for j := 0; j < 80; j++ {
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
		aliasMap:        aliases,
		nodes:           nodeMap,
		classifierCache: make([][]string, aliases.Max()),
		classifiers: []NodeClassifier{
			func(node *nodeselection.SelectedNode) string {
				return "email:" + node.Email
			},
		},
	}
	d.classifyNodeAliases()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkProcess(ctx, b, d, segments)
	}
}

func benchmarkProcess(ctx context.Context, b *testing.B, d ObserverFork, segments []rangedloop.Segment) {
	err := d.Process(ctx, segments)
	require.NoError(b, err)
}
