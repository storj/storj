// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestLocal(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	rt := createRoutingTable(teststorj.NodeIDFromString("AA"))
	defer ctx.Check(rt.Close)
	assert.Equal(t, rt.Local().Id.Bytes()[:2], []byte("AA"))
}

func TestK(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	rt := createRoutingTable(teststorj.NodeIDFromString("AA"))
	defer ctx.Check(rt.Close)
	k := rt.K()
	assert.Equal(t, rt.bucketSize, k)

}

func TestCacheSize(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	rt := createRoutingTable(teststorj.NodeIDFromString("AA"))
	defer ctx.Check(rt.Close)
	expected := rt.rcBucketSize
	result := rt.CacheSize()
	assert.Equal(t, expected, result)
}

func TestGetBucket(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	rt := createRoutingTable(teststorj.NodeIDFromString("AA"))
	defer ctx.Check(rt.Close)
	node := teststorj.MockNode("AA")
	node2 := teststorj.MockNode("BB")
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)

	cases := []struct {
		nodeID   storj.NodeID
		expected []*pb.Node
		ok       bool
	}{
		{nodeID: node.Id,
			expected: []*pb.Node{node, node2},
			ok:       true,
		},
		{nodeID: node2.Id,
			expected: []*pb.Node{node, node2},
			ok:       true,
		},
	}
	for i, v := range cases {
		b, e := rt.GetNodes(node2.Id)
		for j, w := range v.expected {
			if !assert.True(t, bytes.Equal(w.Id.Bytes(), b[j].Id.Bytes())) {
				t.Logf("case %v failed expected: ", i)
			}
		}
		if !assert.Equal(t, v.ok, e) {
			t.Logf("case %v failed ok: ", i)
		}
	}
}


func RandomNode() pb.Node {
	node := pb.Node{}
	rand.Read(node.Id[:])
	return node
}
func TestKademliaFindNear(t *testing.T) {
	testFunc := func(t *testing.T, testNodeCount, limit int) {
		selfNode := RandomNode()
		rt := createRoutingTable(selfNode.Id)

		expectedIDs := make([]storj.NodeID, 0)
		for x := 0; x < testNodeCount; x++ {
			n := RandomNode()
			ok, err := rt.addNode(&n)
			require.NoError(t, err)
			if ok { // buckets were full
				expectedIDs = append(expectedIDs, n.Id)
			}
		}
		if testNodeCount > 0 && limit > 0 {
			require.True(t, len(expectedIDs) > 0)
		}
		//makes sure our target is like self, to keep close nodes
		targetNode := pb.Node{Id: selfNode.Id}
		targetNode.Id[storj.NodeIDSize-1] ^= 1 //flip lowest bit
		sortByXOR(expectedIDs, targetNode.Id)

		results, err := rt.FindNear(targetNode.Id, limit)
		require.NoError(t, err)
		counts := []int{len(expectedIDs), limit}
		sort.Ints(counts)
		require.Equal(t, counts[0], len(results))
		for i, result := range results {
			require.Equal(t, (*result).Id.String(), expectedIDs[i].String(), fmt.Sprintf("item %d", i))
		}
	}
	for _, testNodeCount := range []int{0, 1, 10, 100} {
		for _, limit := range []int{0, 1, 10, 100} {
			t.Run(fmt.Sprintf("test %d %d", testNodeCount, limit),
				func(t *testing.T) { testFunc(t, testNodeCount, limit) })
		}
	}
}

func TestConnectionSuccess(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	id := teststorj.NodeIDFromString("AA")
	rt := createRoutingTable(id)
	defer ctx.Check(rt.Close)
	id2 := teststorj.NodeIDFromString("BB")
	address1 := &pb.NodeAddress{Address: "a"}
	address2 := &pb.NodeAddress{Address: "b"}
	node1 := &pb.Node{Id: id, Address: address1, Type: pb.NodeType_STORAGE}
	node2 := &pb.Node{Id: id2, Address: address2, Type: pb.NodeType_STORAGE}
	cases := []struct {
		testID  string
		node    *pb.Node
		id      storj.NodeID
		address *pb.NodeAddress
	}{
		{testID: "Update Node",
			node:    node1,
			id:      id,
			address: address1,
		},
		{testID: "Create Node",
			node:    node2,
			id:      id2,
			address: address2,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			err := rt.ConnectionSuccess(c.node)
			assert.NoError(t, err)
			v, err := rt.nodeBucketDB.Get(c.id.Bytes())
			assert.NoError(t, err)
			n, err := unmarshalNodes([]storage.Value{v})
			assert.NoError(t, err)
			assert.Equal(t, c.address.Address, n[0].Address.Address)
		})
	}
}

func TestUpdateSelf(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	id := teststorj.NodeIDFromString("AA")
	rt := createRoutingTable(id)
	defer ctx.Check(rt.Close)
	address := &pb.NodeAddress{Address: "a"}
	node := &pb.Node{Id: id, Address: address, Type: pb.NodeType_STORAGE}
	cases := []struct {
		testID  string
		node    *pb.Node
		id      storj.NodeID
		address *pb.NodeAddress
	}{
		{testID: "Update Node",
			node:    node,
			id:      id,
			address: address,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			newNode := c.node
			restrictions := &pb.NodeRestrictions{
				FreeBandwidth: 10,
			}
			newNode.Restrictions = restrictions
			err := rt.UpdateSelf(newNode)
			assert.NoError(t, err)
			v, err := rt.nodeBucketDB.Get(c.id.Bytes())
			assert.NoError(t, err)
			n, err := unmarshalNodes([]storage.Value{v})
			assert.NoError(t, err)
			assert.Equal(t, c.address.Address, n[0].Address.Address)
			assert.Equal(t, newNode.Restrictions.GetFreeBandwidth(), n[0].Restrictions.GetFreeBandwidth())
		})
	}
}

func TestConnectionFailed(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	id := teststorj.NodeIDFromString("AA")
	node := &pb.Node{Id: id, Type: pb.NodeType_STORAGE}
	rt := createRoutingTable(id)
	defer ctx.Check(rt.Close)
	err := rt.ConnectionFailed(node)
	assert.NoError(t, err)
	v, err := rt.nodeBucketDB.Get(id.Bytes())
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestSetBucketTimestamp(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	id := teststorj.NodeIDFromString("AA")
	rt := createRoutingTable(id)
	defer ctx.Check(rt.Close)
	now := time.Now().UTC()

	err := rt.createOrUpdateKBucket(keyToBucketID(id.Bytes()), now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(id.Bytes())
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
	now = time.Now().UTC()
	err = rt.SetBucketTimestamp(id.Bytes(), now)
	assert.NoError(t, err)
	ti, err = rt.GetBucketTimestamp(id.Bytes())
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}

func TestGetBucketTimestamp(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	id := teststorj.NodeIDFromString("AA")
	rt := createRoutingTable(id)
	defer ctx.Check(rt.Close)
	now := time.Now().UTC()
	err := rt.createOrUpdateKBucket(keyToBucketID(id.Bytes()), now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(id.Bytes())
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}
