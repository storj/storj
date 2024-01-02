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

	segment := func(nodes []*nodeselection.SelectedNode, ix ...int) (res rangedloop.Segment) {
		var pieces metabase.AliasPieces
		for n, i := range ix {
			pieces = append(pieces, metabase.AliasPiece{
				Number: uint16(n),
				Alias:  metabase.NodeAlias(i),
			})
		}

		res.StreamID = testrand.UUID()
		res.Position = metabase.SegmentPosition{
			Part:  0,
			Index: 0,
		}

		// it's not inline if non-default redundancy is set.
		res.Redundancy = storj.RedundancyScheme{
			ShareSize: 123,
		}

		res.AliasPieces = pieces

		return res
	}

	ctx := testcontext.New(t)
	c := NewDurability(nil, nil, "net", func(node *nodeselection.SelectedNode) string {
		return node.LastNet
	}, 110, 0, 0, 0)

	c.aliasMap = metabase.NewNodeAliasMap(aliases)
	for _, node := range storageNodes {
		c.nodes[node.ID] = node
	}

	c.classifyNodeAliases()

	fork, err := c.Fork(ctx)
	require.NoError(t, err)

	segment1 := segment(storageNodes, 3, 6, 9, 1)
	{
		// first batch
		err = fork.Process(ctx, []rangedloop.Segment{
			segment1,
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

	require.NotNil(t, c.healthStat["127.0.0.0"])
	require.Equal(t, 1, c.healthStat["127.0.0.0"].Min())
	require.Equal(t, segment1.StreamID.String()+"/0", c.healthStat["127.0.0.0"].Exemplar)
	require.Equal(t, 2, c.healthStat["127.0.1.0"].Min())
	require.Equal(t, 3, c.healthStat["127.0.2.0"].Min())

	// usually called with c.Start()
	c.resetStat()

	fork, err = c.Fork(ctx)
	require.NoError(t, err)
	err = c.Join(ctx, fork)
	require.NoError(t, err)

	// second run supposed to have zero stat.
	require.Nil(t, c.healthStat["127.0.0.0"])
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
	c := NewDurability(nil, nil, "net", func(node *nodeselection.SelectedNode) string {
		return node.LastNet
	}, 110, 0, 0, 0)

	c.aliasMap = metabase.NewNodeAliasMap(aliases)
	for _, node := range storageNodes {
		c.nodes[node.ID] = node
	}

	c.classifyNodeAliases()
	fork, err := c.Fork(ctx)
	require.NoError(t, err)

	// note: second piece points to an alias which was not preloaded (newly inserted).
	err = fork.Process(ctx, []rangedloop.Segment{
		{
			StreamID: testrand.UUID(),
			Position: metabase.SegmentPosition{
				Part:  0,
				Index: 0,
			},
			Redundancy: storj.RedundancyScheme{
				ShareSize: 123,
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
	require.Equal(t, 0, c.healthStat["127.0.0.1"].Min())
}

func TestBusFactor(t *testing.T) {
	ctx := testcontext.New(t)
	f := ObserverFork{}

	for i := 0; i < 100; i++ {
		f.classified = append(f.classified, classID(i))
	}
	f.controlledByClassCache = make([]int32, 100)
	f.busFactorCache = make([]int32, 300)
	f.healthStat = make([]HealthStat, 100)
	f.busFactorThreshold = 26

	createSegments := func(groups ...int) []rangedloop.Segment {
		var pieces []metabase.AliasPiece
		ix := uint16(0)
		groupIndex := 0
		for _, group := range groups {
			for i := 0; i < group; i++ {
				pieces = append(pieces, metabase.AliasPiece{
					Number: ix,
					Alias:  metabase.NodeAlias(groupIndex),
				})
				ix++
			}
			groupIndex++
		}
		return []rangedloop.Segment{
			{
				StreamID:    testrand.UUID(),
				AliasPieces: pieces,
				Redundancy: storj.RedundancyScheme{
					ShareSize: 123,
				},
			},
		}
	}

	err := f.Process(ctx, createSegments(10, 10, 10, 10, 5, 5, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1))
	require.NoError(t, err)
	require.Equal(t, 3, f.busFactor.Min())
}

func BenchmarkDurabilityProcess(b *testing.B) {
	ctx := context.TODO()

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

	d := ObserverFork{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkProcess(ctx, b, d, segments)
	}
}

func benchmarkProcess(ctx context.Context, b *testing.B, d ObserverFork, segments []rangedloop.Segment) {
	err := d.Process(ctx, segments)
	require.NoError(b, err)
}
