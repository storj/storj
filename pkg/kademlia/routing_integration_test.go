// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func newRouting(self storj.NodeID, bucketSize, cacheSize int) (dht.RoutingTable, func()) {
	return createRoutingTableWith(self, routingTableOpts{
		bucketSize: bucketSize,
		cacheSize:  cacheSize,
	})
}

func TestTableInit(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	bucketSize := 5
	cacheSize := 3
	table, close := newRouting(PadID("55", "5"), bucketSize, cacheSize)
	defer close()
	require.Equal(t, bucketSize, table.K())
	require.Equal(t, cacheSize, table.CacheSize())

	nodes, err := table.FindNear(PadID("20", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 0, len(nodes))
}

func TestTableBasic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("5555", "5"), 5, 3)
	defer close()

	err := table.ConnectionSuccess(Node(PadID("5556", "5"), "address:1"))
	require.NoError(t, err)

	nodes, err := table.FindNear(PadID("20", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodes))
	require.Equal(t, PadID("5556", "5"), nodes[0].Id)
	require.Equal(t, "address:1", nodes[0].Address.Address)
}

func TestNoSelf(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("55", "5"), 5, 3)
	defer close()
	err := table.ConnectionSuccess(Node(PadID("55", "5"), "address:2"))
	require.NoError(t, err)

	nodes, err := table.FindNear(PadID("20", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 0, len(nodes))
}

func TestSplits(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("55", "5"), 5, 2)
	defer close()

	for _, prefix2 := range "08" {
		for _, prefix1 := range "a69c23f1d7eb5408" {
			require.NoError(t, table.ConnectionSuccess(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// we just put 32 nodes into the table. the bucket with a differing first
	// bit should be full with 5 nodes. the bucket with the same first bit and
	// differing second bit should be full with 5 nodes. the bucket with the
	// same first two bits and differing third bit should not be full and have
	// 4 nodes (60..., 68..., 70..., 78...). the bucket with the same first
	// three bits should also not be full and have 4 nodes
	// (40..., 48..., 50..., 58...). So we should be able to get no more than
	// 18 nodes back

	nodes, err := table.FindNear(PadID("55", "5"), 19)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		// bucket 010 (same first three bits)
		NodeFromPrefix("50", "0"), NodeFromPrefix("58", "0"),
		NodeFromPrefix("40", "0"), NodeFromPrefix("48", "0"),

		// bucket 011 (same first two bits)
		NodeFromPrefix("70", "0"), NodeFromPrefix("78", "0"),
		NodeFromPrefix("60", "0"), NodeFromPrefix("68", "0"),

		// bucket 00 (same first bit)
		NodeFromPrefix("10", "0"),
		NodeFromPrefix("00", "0"),
		NodeFromPrefix("30", "0"),
		// 20 is added first of this group, so it's the only one where there's
		// room for the 28, before this bucket is full
		NodeFromPrefix("20", "0"), NodeFromPrefix("28", "0"),

		// bucket 1 (differing first bit)
		NodeFromPrefix("d0", "0"),
		NodeFromPrefix("c0", "0"),
		NodeFromPrefix("f0", "0"),
		NodeFromPrefix("90", "0"),
		NodeFromPrefix("a0", "0"),
		// e and f were added last so that bucket should have been full by then
	}, nodes)

	// let's cause some failures and make sure the replacement cache fills in
	// the gaps

	// bucket 010 shouldn't have anything in its replacement cache
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("40", "0")))
	// bucket 011 shouldn't have anything in its replacement cache
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("68", "0")))

	// bucket 00 should have two things in its replacement cache, 18... is one of them
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("18", "0")))
	// now just one thing in its replacement cache
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("30", "0")))
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("28", "0")))

	// bucket 1 should have two things in its replacement cache
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("a0", "0")))
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("d0", "0")))
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("90", "0")))

	nodes, err = table.FindNear(PadID("55", "5"), 19)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		// bucket 010
		NodeFromPrefix("50", "0"), NodeFromPrefix("58", "0"),
		NodeFromPrefix("48", "0"),

		// bucket 011
		NodeFromPrefix("70", "0"), NodeFromPrefix("78", "0"),
		NodeFromPrefix("60", "0"),

		// bucket 00
		NodeFromPrefix("10", "0"),
		NodeFromPrefix("00", "0"),
		NodeFromPrefix("08", "0"), // replacement cache
		NodeFromPrefix("20", "0"),

		// bucket 1
		NodeFromPrefix("c0", "0"),
		NodeFromPrefix("f0", "0"),
		NodeFromPrefix("88", "0"), // replacement cache
		NodeFromPrefix("b8", "0"), // replacement cache
	}, nodes)
}

func TestUnbalanced(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("ff", "f"), 5, 2)
	defer close()

	for _, prefix1 := range "0123456789abcdef" {
		for _, prefix2 := range "08" {
			require.NoError(t, table.ConnectionSuccess(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// in this case, we've blown out the routing table with a paradoxical
	// case. every node we added should have been the closest node, so this
	// would have forced every bucket to split, and we should have stored all
	// possible nodes.

	nodes, err := table.FindNear(PadID("ff", "f"), 33)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("f8", "0"), NodeFromPrefix("f0", "0"),
		NodeFromPrefix("e8", "0"), NodeFromPrefix("e0", "0"),
		NodeFromPrefix("d8", "0"), NodeFromPrefix("d0", "0"),
		NodeFromPrefix("c8", "0"), NodeFromPrefix("c0", "0"),
		NodeFromPrefix("b8", "0"), NodeFromPrefix("b0", "0"),
		NodeFromPrefix("a8", "0"), NodeFromPrefix("a0", "0"),
		NodeFromPrefix("98", "0"), NodeFromPrefix("90", "0"),
		NodeFromPrefix("88", "0"), NodeFromPrefix("80", "0"),
		NodeFromPrefix("78", "0"), NodeFromPrefix("70", "0"),
		NodeFromPrefix("68", "0"), NodeFromPrefix("60", "0"),
		NodeFromPrefix("58", "0"), NodeFromPrefix("50", "0"),
		NodeFromPrefix("48", "0"), NodeFromPrefix("40", "0"),
		NodeFromPrefix("38", "0"), NodeFromPrefix("30", "0"),
		NodeFromPrefix("28", "0"), NodeFromPrefix("20", "0"),
		NodeFromPrefix("18", "0"), NodeFromPrefix("10", "0"),
		NodeFromPrefix("08", "0"), NodeFromPrefix("00", "0"),
	}, nodes)
}

func TestQuery(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("a3", "3"), 5, 2)
	defer close()

	for _, prefix2 := range "08" {
		for _, prefix1 := range "b4f25c896de03a71" {
			require.NoError(t, table.ConnectionSuccess(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "f")))
		}
	}

	nodes, err := table.FindNear(PadID("c7139", "1"), 2)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c0", "f"),
		NodeFromPrefix("d0", "f"),
	}, nodes)

	nodes, err = table.FindNear(PadID("c7139", "1"), 7)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c0", "f"),
		NodeFromPrefix("d0", "f"),
		NodeFromPrefix("e0", "f"),
		NodeFromPrefix("f0", "f"),
		NodeFromPrefix("f8", "f"),
		NodeFromPrefix("80", "f"),
		NodeFromPrefix("88", "f"),
	}, nodes)

	nodes, err = table.FindNear(PadID("c7139", "1"), 10)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c0", "f"),
		NodeFromPrefix("d0", "f"),
		NodeFromPrefix("e0", "f"),
		NodeFromPrefix("f0", "f"),
		NodeFromPrefix("f8", "f"),
		NodeFromPrefix("80", "f"),
		NodeFromPrefix("88", "f"),
		NodeFromPrefix("90", "f"),
		NodeFromPrefix("98", "f"),
		NodeFromPrefix("a0", "f"),
	}, nodes)
}

func TestUpdateBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("a3", "3"), 5, 2)
	defer close()

	for _, prefix2 := range "08" {
		for _, prefix1 := range "b4f25c896de03a71" {
			require.NoError(t, table.ConnectionSuccess(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "f")))
		}
	}

	nodes, err := table.FindNear(PadID("c7139", "1"), 1)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c0", "f"),
	}, nodes)

	require.NoError(t, table.ConnectionSuccess(
		Node(PadID("c0", "f"), "new-address:3")))

	nodes, err = table.FindNear(PadID("c7139", "1"), 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodes))
	require.Equal(t, PadID("c0", "f"), nodes[0].Id)
	require.Equal(t, "new-address:3", nodes[0].Address.Address)
}

func TestUpdateCache(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("a3", "3"), 1, 1)
	defer close()

	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("80", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("c0", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("40", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("00", "0")))

	require.NoError(t, table.ConnectionSuccess(Node(PadID("00", "0"), "new-address:6")))
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("40", "0")))

	nodes, err := table.FindNear(PadID("00", "0"), 4)
	require.NoError(t, err)

	requireNodesEqual(t, []*pb.Node{
		Node(PadID("00", "0"), "new-address:6"),
		NodeFromPrefix("80", "0"),
		NodeFromPrefix("c0", "0"),
	}, nodes)
}

func TestFailureUnknownAddress(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("a3", "3"), 1, 1)
	defer close()

	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("80", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("c0", "0")))
	require.NoError(t, table.ConnectionSuccess(Node(PadID("40", "0"), "address:2")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("00", "0")))
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("40", "0")))

	nodes, err := table.FindNear(PadID("00", "0"), 4)
	require.NoError(t, err)

	requireNodesEqual(t, []*pb.Node{
		Node(PadID("40", "0"), "address:2"),
		NodeFromPrefix("80", "0"),
		NodeFromPrefix("c0", "0"),
	}, nodes)
}

func TestShrink(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := newRouting(PadID("ff", "f"), 2, 2)
	defer close()

	// blow out the routing table
	for _, prefix1 := range "0123456789abcdef" {
		for _, prefix2 := range "08" {
			require.NoError(t, table.ConnectionSuccess(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// delete some of the bad ones
	for _, prefix1 := range "0123456789abcd" {
		for _, prefix2 := range "08" {
			require.NoError(t, table.ConnectionFailed(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// add back some nodes more balanced
	for _, prefix1 := range "3a50" {
		for _, prefix2 := range "19" {
			require.NoError(t, table.ConnectionSuccess(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// make sure table filled in alright
	nodes, err := table.FindNear(PadID("ff", "f"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("f8", "0"),
		NodeFromPrefix("f0", "0"),
		NodeFromPrefix("e8", "0"),
		NodeFromPrefix("e0", "0"),
		NodeFromPrefix("a9", "0"),
		NodeFromPrefix("a1", "0"),
		NodeFromPrefix("39", "0"),
		NodeFromPrefix("31", "0"),
	}, nodes)
}
