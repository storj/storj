// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/kademlia/testrouting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// RoutingTableInterface contains information on nodes we have locally
type RoutingTableInterface interface {
	K() int
	CacheSize() int
	FindNear(ctx context.Context, id storj.NodeID, limit int) ([]*pb.Node, error)
	ConnectionSuccess(ctx context.Context, node *pb.Node) error
	ConnectionFailed(ctx context.Context, node *pb.Node) error
	Close() error
}

type routingCtor func(context.Context, storj.NodeID, int, int, int) RoutingTableInterface

func newRouting(ctx context.Context, self storj.NodeID, bucketSize, cacheSize, allowedFailures int) RoutingTableInterface {
	if allowedFailures != 0 {
		panic("failure counting currently unsupported")
	}
	return createRoutingTableWith(ctx, self, routingTableOpts{
		bucketSize: bucketSize,
		cacheSize:  cacheSize,
	})
}

func newTestRouting(ctx context.Context, self storj.NodeID, bucketSize, cacheSize, allowedFailures int) RoutingTableInterface {
	return testrouting.New(self, bucketSize, cacheSize, allowedFailures)
}

func TestTableInit_Routing(t *testing.T)     { testTableInit(t, newRouting) }
func TestTableInit_TestRouting(t *testing.T) { testTableInit(t, newTestRouting) }
func testTableInit(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	bucketSize := 5
	cacheSize := 3
	table := routingCtor(ctx, PadID("55", "5"), bucketSize, cacheSize, 0)
	defer ctx.Check(table.Close)
	require.Equal(t, bucketSize, table.K())
	require.Equal(t, cacheSize, table.CacheSize())

	nodes, err := table.FindNear(ctx, PadID("21", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 0, len(nodes))
}

func TestTableBasic_Routing(t *testing.T)     { testTableBasic(t, newRouting) }
func TestTableBasic_TestRouting(t *testing.T) { testTableBasic(t, newTestRouting) }
func testTableBasic(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("5555", "5"), 5, 3, 0)
	defer ctx.Check(table.Close)

	err := table.ConnectionSuccess(ctx, Node(PadID("5556", "5"), "address:1"))
	require.NoError(t, err)

	nodes, err := table.FindNear(ctx, PadID("21", "0"), 3)
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

	table := routingCtor(ctx, PadID("55", "5"), 5, 3, 0)
	defer ctx.Check(table.Close)
	err := table.ConnectionSuccess(ctx, Node(PadID("55", "5"), "address:2"))
	require.NoError(t, err)

	nodes, err := table.FindNear(ctx, PadID("21", "0"), 3)
	require.NoError(t, err)
	require.Equal(t, 0, len(nodes))
}

func TestSplits_Routing(t *testing.T)     { testSplits(t, newRouting) }
func TestSplits_TestRouting(t *testing.T) { testSplits(t, newTestRouting) }
func testSplits(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("55", "5"), 5, 2, 0)
	defer ctx.Check(table.Close)

	for _, prefix2 := range "18" {
		for _, prefix1 := range "a69c23f1d7eb5408" {
			require.NoError(t, table.ConnectionSuccess(ctx,
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
	nodes, err := table.FindNear(ctx, PadID("55", "5"), 19)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		// bucket 010 (same first three bits)
		NodeFromPrefix("51", "0"), NodeFromPrefix("58", "0"),
		NodeFromPrefix("41", "0"), NodeFromPrefix("48", "0"),

		// bucket 011 (same first two bits)
		NodeFromPrefix("71", "0"), NodeFromPrefix("78", "0"),
		NodeFromPrefix("61", "0"), NodeFromPrefix("68", "0"),

		// bucket 00 (same first bit)
		NodeFromPrefix("11", "0"),
		NodeFromPrefix("01", "0"),
		NodeFromPrefix("31", "0"),
		// 20 is added first of this group, so it's the only one where there's
		// room for the 28, before this bucket is full
		NodeFromPrefix("21", "0"), NodeFromPrefix("28", "0"),

		// bucket 1 (differing first bit)
		NodeFromPrefix("d1", "0"),
		NodeFromPrefix("c1", "0"),
		NodeFromPrefix("f1", "0"),
		NodeFromPrefix("91", "0"),
		NodeFromPrefix("a1", "0"),
		// e and f were added last so that bucket should have been full by then
	}, nodes)

	// let's cause some failures and make sure the replacement cache fills in
	// the gaps

	// bucket 010 shouldn't have anything in its replacement cache
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("41", "0")))
	// bucket 011 shouldn't have anything in its replacement cache
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("68", "0")))

	// bucket 00 should have two things in its replacement cache, 18... is one of them
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("18", "0")))

	// now just one thing in its replacement cache
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("31", "0")))
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("28", "0")))

	// bucket 1 should have two things in its replacement cache
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("a1", "0")))
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("d1", "0")))
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("91", "0")))

	nodes, err = table.FindNear(ctx, PadID("55", "5"), 19)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		// bucket 010
		NodeFromPrefix("51", "0"), NodeFromPrefix("58", "0"),
		NodeFromPrefix("48", "0"),

		// bucket 011
		NodeFromPrefix("71", "0"), NodeFromPrefix("78", "0"),
		NodeFromPrefix("61", "0"),

		// bucket 00
		NodeFromPrefix("11", "0"),
		NodeFromPrefix("01", "0"),
		NodeFromPrefix("08", "0"), // replacement cache
		NodeFromPrefix("21", "0"),

		// bucket 1
		NodeFromPrefix("c1", "0"),
		NodeFromPrefix("f1", "0"),
		NodeFromPrefix("88", "0"), // replacement cache
		NodeFromPrefix("b8", "0"), // replacement cache
	}, nodes)
}

func TestUnbalanced_Routing(t *testing.T)     { testUnbalanced(t, newRouting) }
func TestUnbalanced_TestRouting(t *testing.T) { testUnbalanced(t, newTestRouting) }
func testUnbalanced(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("ff", "f"), 5, 2, 0)
	defer ctx.Check(table.Close)

	for _, prefix1 := range "0123456789abcdef" {
		for _, prefix2 := range "18" {
			require.NoError(t, table.ConnectionSuccess(ctx,
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// in this case, we've blown out the routing table with a paradoxical
	// case. every node we added should have been the closest node, so this
	// would have forced every bucket to split, and we should have stored all
	// possible nodes.

	nodes, err := table.FindNear(ctx, PadID("ff", "f"), 33)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("f8", "0"), NodeFromPrefix("f1", "0"),
		NodeFromPrefix("e8", "0"), NodeFromPrefix("e1", "0"),
		NodeFromPrefix("d8", "0"), NodeFromPrefix("d1", "0"),
		NodeFromPrefix("c8", "0"), NodeFromPrefix("c1", "0"),
		NodeFromPrefix("b8", "0"), NodeFromPrefix("b1", "0"),
		NodeFromPrefix("a8", "0"), NodeFromPrefix("a1", "0"),
		NodeFromPrefix("98", "0"), NodeFromPrefix("91", "0"),
		NodeFromPrefix("88", "0"), NodeFromPrefix("81", "0"),
		NodeFromPrefix("78", "0"), NodeFromPrefix("71", "0"),
		NodeFromPrefix("68", "0"), NodeFromPrefix("61", "0"),
		NodeFromPrefix("58", "0"), NodeFromPrefix("51", "0"),
		NodeFromPrefix("48", "0"), NodeFromPrefix("41", "0"),
		NodeFromPrefix("38", "0"), NodeFromPrefix("31", "0"),
		NodeFromPrefix("28", "0"), NodeFromPrefix("21", "0"),
		NodeFromPrefix("18", "0"), NodeFromPrefix("11", "0"),
		NodeFromPrefix("08", "0"), NodeFromPrefix("01", "0"),
	}, nodes)
}

func TestQuery_Routing(t *testing.T)     { testQuery(t, newRouting) }
func TestQuery_TestRouting(t *testing.T) { testQuery(t, newTestRouting) }
func testQuery(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("a3", "3"), 5, 2, 0)
	defer ctx.Check(table.Close)

	for _, prefix2 := range "18" {
		for _, prefix1 := range "b4f25c896de03a71" {
			require.NoError(t, table.ConnectionSuccess(ctx,
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "f")))
		}
	}

	nodes, err := table.FindNear(ctx, PadID("c7139", "1"), 2)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c1", "f"),
		NodeFromPrefix("d1", "f"),
	}, nodes)

	nodes, err = table.FindNear(ctx, PadID("c7139", "1"), 7)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c1", "f"),
		NodeFromPrefix("d1", "f"),
		NodeFromPrefix("e1", "f"),
		NodeFromPrefix("f1", "f"),
		NodeFromPrefix("f8", "f"),
		NodeFromPrefix("81", "f"),
		NodeFromPrefix("88", "f"),
	}, nodes)

	nodes, err = table.FindNear(ctx, PadID("c7139", "1"), 10)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c1", "f"),
		NodeFromPrefix("d1", "f"),
		NodeFromPrefix("e1", "f"),
		NodeFromPrefix("f1", "f"),
		NodeFromPrefix("f8", "f"),
		NodeFromPrefix("81", "f"),
		NodeFromPrefix("88", "f"),
		NodeFromPrefix("91", "f"),
		NodeFromPrefix("98", "f"),
		NodeFromPrefix("a1", "f"),
	}, nodes)
}

func TestFailureCounting_Routing(t *testing.T)     { t.Skip() }
func TestFailureCounting_TestRouting(t *testing.T) { testFailureCounting(t, newTestRouting) }
func testFailureCounting(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("a3", "3"), 5, 2, 2)
	defer ctx.Check(table.Close)

	for _, prefix2 := range "18" {
		for _, prefix1 := range "b4f25c896de03a71" {
			require.NoError(t, table.ConnectionSuccess(ctx,
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "f")))
		}
	}

	nochange := func() {
		nodes, err := table.FindNear(ctx, PadID("c7139", "1"), 7)
		require.NoError(t, err)
		requireNodesEqual(t, []*pb.Node{
			NodeFromPrefix("c1", "f"),
			NodeFromPrefix("d1", "f"),
			NodeFromPrefix("e1", "f"),
			NodeFromPrefix("f1", "f"),
			NodeFromPrefix("f8", "f"),
			NodeFromPrefix("81", "f"),
			NodeFromPrefix("88", "f"),
		}, nodes)
	}

	nochange()
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("d1", "f")))
	nochange()
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("d1", "f")))
	nochange()
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("d1", "f")))

	nodes, err := table.FindNear(ctx, PadID("c7139", "1"), 7)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c1", "f"),
		NodeFromPrefix("e1", "f"),
		NodeFromPrefix("e8", "f"),
		NodeFromPrefix("f1", "f"),
		NodeFromPrefix("f8", "f"),
		NodeFromPrefix("81", "f"),
		NodeFromPrefix("88", "f"),
	}, nodes)
}

func TestUpdateBucket_Routing(t *testing.T)     { testUpdateBucket(t, newRouting) }
func TestUpdateBucket_TestRouting(t *testing.T) { testUpdateBucket(t, newTestRouting) }
func testUpdateBucket(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("a3", "3"), 5, 2, 0)
	defer ctx.Check(table.Close)

	for _, prefix2 := range "18" {
		for _, prefix1 := range "b4f25c896de03a71" {
			require.NoError(t, table.ConnectionSuccess(ctx,
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "f")))
		}
	}

	nodes, err := table.FindNear(ctx, PadID("c7139", "1"), 1)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("c1", "f"),
	}, nodes)

	require.NoError(t, table.ConnectionSuccess(ctx,
		Node(PadID("c1", "f"), "new-address:3")))

	nodes, err = table.FindNear(ctx, PadID("c7139", "1"), 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodes))
	require.Equal(t, PadID("c1", "f"), nodes[0].Id)
	require.Equal(t, "new-address:3", nodes[0].Address.Address)
}

func TestUpdateCache_Routing(t *testing.T)     { testUpdateCache(t, newRouting) }
func TestUpdateCache_TestRouting(t *testing.T) { testUpdateCache(t, newTestRouting) }
func testUpdateCache(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("a3", "3"), 1, 1, 0)
	defer ctx.Check(table.Close)

	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("81", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("c1", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("41", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("01", "0")))

	require.NoError(t, table.ConnectionSuccess(ctx, Node(PadID("01", "0"), "new-address:6")))
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("41", "0")))

	nodes, err := table.FindNear(ctx, PadID("01", "0"), 4)
	require.NoError(t, err)

	requireNodesEqual(t, []*pb.Node{
		Node(PadID("01", "0"), "new-address:6"),
		NodeFromPrefix("81", "0"),
		NodeFromPrefix("c1", "0"),
	}, nodes)
}

func TestFailureUnknownAddress_Routing(t *testing.T)     { testFailureUnknownAddress(t, newRouting) }
func TestFailureUnknownAddress_TestRouting(t *testing.T) { testFailureUnknownAddress(t, newTestRouting) }
func testFailureUnknownAddress(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("a3", "3"), 1, 1, 0)
	defer ctx.Check(table.Close)

	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("81", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("c1", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, Node(PadID("41", "0"), "address:2")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("01", "0")))
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("41", "0")))

	nodes, err := table.FindNear(ctx, PadID("01", "0"), 4)
	require.NoError(t, err)

	requireNodesEqual(t, []*pb.Node{
		Node(PadID("41", "0"), "address:2"),
		NodeFromPrefix("81", "0"),
		NodeFromPrefix("c1", "0"),
	}, nodes)
}

func TestShrink_Routing(t *testing.T)     { testShrink(t, newRouting) }
func TestShrink_TestRouting(t *testing.T) { testShrink(t, newTestRouting) }
func testShrink(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("ff", "f"), 2, 2, 0)
	defer ctx.Check(table.Close)

	// blow out the routing table
	for _, prefix1 := range "0123456789abcdef" {
		for _, prefix2 := range "18" {
			require.NoError(t, table.ConnectionSuccess(ctx,
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// delete some of the bad ones
	for _, prefix1 := range "0123456789abcd" {
		for _, prefix2 := range "18" {
			require.NoError(t, table.ConnectionFailed(ctx,
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// add back some nodes more balanced
	for _, prefix1 := range "3a50" {
		for _, prefix2 := range "19" {
			require.NoError(t, table.ConnectionSuccess(ctx,
				NodeFromPrefix(string([]rune{prefix1, prefix2}), "0")))
		}
	}

	// make sure table filled in alright
	nodes, err := table.FindNear(ctx, PadID("ff", "f"), 13)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("f8", "0"),
		NodeFromPrefix("f1", "0"),
		NodeFromPrefix("e8", "0"),
		NodeFromPrefix("e1", "0"),
		NodeFromPrefix("a9", "0"),
		NodeFromPrefix("a1", "0"),
		NodeFromPrefix("59", "0"),
		NodeFromPrefix("51", "0"),
		NodeFromPrefix("39", "0"),
		NodeFromPrefix("31", "0"),
		NodeFromPrefix("09", "0"),
		NodeFromPrefix("01", "0"),
	}, nodes)
}

func TestReplacementCacheOrder_Routing(t *testing.T)     { testReplacementCacheOrder(t, newRouting) }
func TestReplacementCacheOrder_TestRouting(t *testing.T) { testReplacementCacheOrder(t, newTestRouting) }
func testReplacementCacheOrder(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("a3", "3"), 1, 2, 0)
	defer ctx.Check(table.Close)

	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("81", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("21", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("c1", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("41", "0")))
	require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix("01", "0")))
	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("21", "0")))

	nodes, err := table.FindNear(ctx, PadID("55", "5"), 4)
	require.NoError(t, err)

	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("01", "0"),
		NodeFromPrefix("c1", "0"),
		NodeFromPrefix("81", "0"),
	}, nodes)
}

func TestHealSplit_Routing(t *testing.T)     { testHealSplit(t, newRouting) }
func TestHealSplit_TestRouting(t *testing.T) { testHealSplit(t, newTestRouting) }
func testHealSplit(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("55", "55"), 2, 2, 0)
	defer ctx.Check(table.Close)

	for _, pad := range []string{"0", "1"} {
		for _, prefix := range []string{"ff", "e1", "c1", "54", "56", "57"} {
			require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix(prefix, pad)))
		}
	}

	nodes, err := table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c1", "1"),
		NodeFromPrefix("c1", "0"),
		NodeFromPrefix("ff", "0"),
		NodeFromPrefix("e1", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("c1", "0")))

	nodes, err = table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c1", "1"),
		NodeFromPrefix("ff", "0"),
		NodeFromPrefix("e1", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("ff", "0")))
	nodes, err = table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c1", "1"),
		NodeFromPrefix("e1", "1"),
		NodeFromPrefix("e1", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("e1", "0")))
	nodes, err = table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c1", "1"),
		NodeFromPrefix("ff", "1"),
		NodeFromPrefix("e1", "1"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("e1", "1")))
	nodes, err = table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c1", "1"),
		NodeFromPrefix("ff", "1"),
	}, nodes)

	for _, prefix := range []string{"ff", "e1", "c1", "54", "56", "57"} {
		require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix(prefix, "2")))
	}

	nodes, err = table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("54", "1"),
		NodeFromPrefix("54", "0"),
		NodeFromPrefix("57", "0"),
		NodeFromPrefix("56", "0"),
		NodeFromPrefix("c1", "1"),
		NodeFromPrefix("c1", "2"),
		NodeFromPrefix("ff", "1"),
		NodeFromPrefix("ff", "2"),
	}, nodes)
}

func TestFullDissimilarBucket_Routing(t *testing.T)     { testFullDissimilarBucket(t, newRouting) }
func TestFullDissimilarBucket_TestRouting(t *testing.T) { testFullDissimilarBucket(t, newTestRouting) }
func testFullDissimilarBucket(t *testing.T, routingCtor routingCtor) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	table := routingCtor(ctx, PadID("55", "55"), 2, 2, 0)
	defer ctx.Check(table.Close)

	for _, prefix := range []string{"d1", "c1", "f1", "e1"} {
		require.NoError(t, table.ConnectionSuccess(ctx, NodeFromPrefix(prefix, "0")))
	}

	nodes, err := table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("d1", "0"),
		NodeFromPrefix("c1", "0"),
	}, nodes)

	require.NoError(t, table.ConnectionFailed(ctx, NodeFromPrefix("c1", "0")))

	nodes, err = table.FindNear(ctx, PadID("55", "55"), 9)
	require.NoError(t, err)
	requireNodesEqual(t, []*pb.Node{
		NodeFromPrefix("d1", "0"),
		NodeFromPrefix("e1", "0"),
	}, nodes)
}
