// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia/testrouting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type routingCtor func(storj.NodeID, int, int, int) (dht.RoutingTable, func())

func newRouting(self storj.NodeID, bucketSize, cacheSize, allowedFailures int) (dht.RoutingTable, func()) {
	if allowedFailures != 0 {
		panic("failure counting currently unsupported")
	}
	return createRoutingTableWith(self, routingTableOpts{
		bucketSize: bucketSize,
		cacheSize:  cacheSize,
	})
}

func newTestRouting(self storj.NodeID, bucketSize, cacheSize, allowedFailures int) (dht.RoutingTable, func()) {
	return testrouting.New(self, bucketSize, cacheSize, allowedFailures), func() {}
}

func TestTableInit_Routing(t *testing.T)     { testTableInit(t, newRouting) }
func TestTableInit_TestRouting(t *testing.T) { testTableInit(t, newTestRouting) }
func testTableInit(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	bucketSize := 5
	cacheSize := 3
	table, close := routingCtor(PadID("55", "5"), bucketSize, cacheSize, 0)
	defer close()
	require.Equal(t, bucketSize, table.K())
	require.Equal(t, cacheSize, table.CacheSize())

	nodes, err := table.FindNear(PadID("20", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 0, len(nodes))
}

func TestTableBasic_Routing(t *testing.T)     { testTableBasic(t, newRouting) }
func TestTableBasic_TestRouting(t *testing.T) { testTableBasic(t, newTestRouting) }
func testTableBasic(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("5555", "5"), 5, 3, 0)
	defer close()

	err := table.ConnectionSuccess(Node(PadID("5556", "5"), "address:1"))
	require.NoError(t, err)

	nodes, err := table.FindNear(PadID("20", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodes))
	require.Equal(t, PadID("5556", "5"), nodes[0].Id)
	require.Equal(t, "address:1", nodes[0].Address.Address)
}

func TestNoSelf_Routing(t *testing.T)     { testNoSelf(t, newRouting) }
func TestNoSelf_TestRouting(t *testing.T) { testNoSelf(t, newTestRouting) }
func testNoSelf(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("55", "5"), 5, 3, 0)
	defer close()
	err := table.ConnectionSuccess(Node(PadID("55", "5"), "address:2"))
	require.NoError(t, err)

	nodes, err := table.FindNear(PadID("20", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 0, len(nodes))
}

func TestSplits_Routing(t *testing.T)     { testSplits(t, newRouting) }
func TestSplits_TestRouting(t *testing.T) { testSplits(t, newTestRouting) }
func testSplits(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("55", "5"), 5, 2, 0)
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

func TestUnbalanced_Routing(t *testing.T)     { testUnbalanced(t, newRouting) }
func TestUnbalanced_TestRouting(t *testing.T) { testUnbalanced(t, newTestRouting) }
func testUnbalanced(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("ff", "f"), 5, 2, 0)
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

func TestQuery_Routing(t *testing.T)     { testQuery(t, newRouting) }
func TestQuery_TestRouting(t *testing.T) { testQuery(t, newTestRouting) }
func testQuery(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("a3", "3"), 5, 2, 0)
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

func TestFailureCounting_Routing(t *testing.T)     { t.Skip() }
func TestFailureCounting_TestRouting(t *testing.T) { testFailureCounting(t, newTestRouting) }
func testFailureCounting(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("a3", "3"), 5, 2, 2)
	defer close()

	for _, prefix2 := range "08" {
		for _, prefix1 := range "b4f25c896de03a71" {
			require.NoError(t, table.ConnectionSuccess(
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "f")))
		}
	}

	nochange := func() {
		nodes, err := table.FindNear(PadID("c7139", "1"), 7)
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
	}

	nochange()
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("d0", "f")))
	nochange()
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("d0", "f")))
	nochange()
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("d0", "f")))

	nodes, err := table.FindNear(PadID("c7139", "1"), 7)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c0", "f"),
		NodeFromPrefix("e0", "f"),
		NodeFromPrefix("e8", "f"),
		NodeFromPrefix("f0", "f"),
		NodeFromPrefix("f8", "f"),
		NodeFromPrefix("80", "f"),
		NodeFromPrefix("88", "f"),
	}, nodes)
}

func TestUpdateBucket_Routing(t *testing.T)     { testUpdateBucket(t, newRouting) }
func TestUpdateBucket_TestRouting(t *testing.T) { testUpdateBucket(t, newTestRouting) }
func testUpdateBucket(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("a3", "3"), 5, 2, 0)
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

func TestUpdateCache_Routing(t *testing.T)     { testUpdateCache(t, newRouting) }
func TestUpdateCache_TestRouting(t *testing.T) { testUpdateCache(t, newTestRouting) }
func testUpdateCache(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("a3", "3"), 1, 1, 0)
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

func TestFailureUnknownAddress_Routing(t *testing.T)     { testFailureUnknownAddress(t, newRouting) }
func TestFailureUnknownAddress_TestRouting(t *testing.T) { testFailureUnknownAddress(t, newTestRouting) }
func testFailureUnknownAddress(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("a3", "3"), 1, 1, 0)
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

func TestShrink_Routing(t *testing.T)     { testShrink(t, newRouting) }
func TestShrink_TestRouting(t *testing.T) { testShrink(t, newTestRouting) }
func testShrink(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("ff", "f"), 2, 2, 0)
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

func TestReplacementCacheOrder_Routing(t *testing.T)     { testReplacementCacheOrder(t, newRouting) }
func TestReplacementCacheOrder_TestRouting(t *testing.T) { testReplacementCacheOrder(t, newTestRouting) }
func testReplacementCacheOrder(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("a3", "3"), 1, 2, 0)
	defer close()

	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("80", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("20", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("c0", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("40", "0")))
	require.NoError(t, table.ConnectionSuccess(NodeFromPrefix("00", "0")))
	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("20", "0")))

	nodes, err := table.FindNear(PadID("55", "5"), 4)
	require.NoError(t, err)

	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("00", "0"),
		NodeFromPrefix("c0", "0"),
		NodeFromPrefix("80", "0"),
	}, nodes)
}

func TestHealSplit_Routing(t *testing.T)     { testHealSplit(t, newRouting) }
func TestHealSplit_TestRouting(t *testing.T) { testHealSplit(t, newTestRouting) }
func testHealSplit(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("55", "55"), 2, 2, 0)
	defer close()

	for _, pad := range []string{"0", "1"} {
		for _, prefix := range []string{"ff", "e0", "c0", "54", "56", "57"} {
			require.NoError(t, table.ConnectionSuccess(NodeFromPrefix(prefix, pad)))
		}
	}

	nodes, err := table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c0", "1"),
		NodeFromPrefix("c0", "0"),
		NodeFromPrefix("ff", "0"),
		NodeFromPrefix("e0", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("c0", "0")))

	nodes, err = table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("ff", "0"),
		NodeFromPrefix("e0", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("ff", "0")))
	nodes, err = table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c0", "1"),
		NodeFromPrefix("e0", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("e0", "0")))
	nodes, err = table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c0", "1"),
		NodeFromPrefix("e0", "1"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("e0", "1")))
	nodes, err = table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c0", "1"),
	}, nodes)

	for _, prefix := range []string{"ff", "e0", "c0", "54", "56", "57"} {
		require.NoError(t, table.ConnectionSuccess(NodeFromPrefix(prefix, "2")))
	}

	nodes, err = table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c0", "1"),
		NodeFromPrefix("ff", "2"),
	}, nodes)
}

func TestFullDissimilarBucket_Routing(t *testing.T)     { testFullDissimilarBucket(t, newRouting) }
func TestFullDissimilarBucket_TestRouting(t *testing.T) { testFullDissimilarBucket(t, newTestRouting) }
func testFullDissimilarBucket(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table, close := routingCtor(PadID("55", "55"), 2, 2, 0)
	defer close()

	for _, prefix := range []string{"d0", "c0", "f0", "e0"} {
		require.NoError(t, table.ConnectionSuccess(NodeFromPrefix(prefix, "0")))
	}

	nodes, err := table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("d0", "0"),
		NodeFromPrefix("c0", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(NodeFromPrefix("c0", "0")))

	nodes, err = table.FindNear(PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("d0", "0"),
		NodeFromPrefix("e0", "0"),
	}, nodes)
}
