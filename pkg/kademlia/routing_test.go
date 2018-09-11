// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"
	"time"

	pb "github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

func TestLocal(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	local := rt.Local()
	assert.Equal(t, *rt.self, local)
}

func TestK(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	k := rt.K()
	assert.Equal(t, rt.bucketSize, k)

}

func TestCacheSize(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	expected := rt.rcBucketSize
	result := rt.CacheSize()
	assert.Equal(t, expected, result)
}

func TestGetBucket(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	node := mockNode("AA")
	node2 := mockNode("BB")
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)

	cases := []struct {
		nodeID   string
		expected *KBucket
		ok       bool
	}{
		{nodeID: node.Id,
			expected: &KBucket{nodes: []*proto.Node{node, node2}},
			ok:       true,
		},
		{nodeID: node2.Id,
			expected: &KBucket{nodes: []*proto.Node{node, node2}},
			ok:       true,
		},
	}
	for i, v := range cases {
		b, e := rt.GetBucket(node2.Id)
		for j, w := range v.expected.nodes {
			if !assert.True(t, pb.Equal(w, b.Nodes()[j])) {
				t.Logf("case %v failed expected: ", i)
			}
		}
		if !assert.Equal(t, v.ok, e) {
			t.Logf("case %v failed ok: ", i)
		}
	}
}

func TestGetBuckets(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	node := mockNode("AA")
	node2 := mockNode("BB")
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)
	expected := []*proto.Node{node, node2}
	buckets, err := rt.GetBuckets()
	assert.NoError(t, err)
	for _, v := range buckets {
		for j, w := range v.Nodes() {
			assert.True(t, pb.Equal(expected[j], w))
		}
	}
}

func TestFindNear(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	node := mockNode("AA")
	node2 := mockNode("BB")
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)
	expected := []*proto.Node{node}
	nodes, err := rt.FindNear(StringToNodeID(node.Id), 1)
	assert.NoError(t, err)
	assert.Equal(t, expected, nodes)

	node3 := mockNode("CC")
	expected = []*proto.Node{node2, node}
	nodes, err = rt.FindNear(StringToNodeID(node3.Id), 2)
	assert.NoError(t, err)
	assert.Equal(t, expected, nodes)

	expected = []*proto.Node{node2}
	nodes, err = rt.FindNear(StringToNodeID(node3.Id), 1)
	assert.NoError(t, err)
	assert.Equal(t, expected, nodes)

	expected = []*proto.Node{node2, node}
	nodes, err = rt.FindNear(StringToNodeID(node3.Id), 3)
	assert.NoError(t, err)
	assert.Equal(t, expected, nodes)

}

func TestConnectionSuccess(t *testing.T) {
	id := "AA"
	rt, cleanup := createRoutingTable(t, []byte(id))
	defer cleanup()
	id2 := "BB"
	address1 := &proto.NodeAddress{Address: "a"}
	address2 := &proto.NodeAddress{Address: "b"}
	node1 := &proto.Node{Id: id, Address: address1}
	node2 := &proto.Node{Id: id2, Address: address2}

	//Updates node
	err := rt.ConnectionSuccess(node1)
	assert.NoError(t, err)
	v, err := rt.nodeBucketDB.Get([]byte(id))
	assert.NoError(t, err)
	n, err := unmarshalNodes(storage.Keys{storage.Key(id)}, []storage.Value{v})
	assert.NoError(t, err)
	assert.Equal(t, address1.Address, n[0].Address.Address)

	//Add Node
	err = rt.ConnectionSuccess(node2)
	assert.NoError(t, err)
	v, err = rt.nodeBucketDB.Get([]byte(id2))
	assert.NoError(t, err)
	n, err = unmarshalNodes(storage.Keys{storage.Key(id2)}, []storage.Value{v})
	assert.NoError(t, err)
	assert.Equal(t, address2.Address, n[0].Address.Address)
}

func TestConnectionFailed(t *testing.T) {
	id := "AA"
	node := mockNode(id)
	rt, cleanup := createRoutingTable(t, []byte(id))
	defer cleanup()
	err := rt.ConnectionFailed(node)
	assert.NoError(t, err)
	v, err := rt.nodeBucketDB.Get([]byte(id))
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestSetBucketTimestamp(t *testing.T) {
	id := []byte("AA")
	idStr := string(id)
	rt, cleanup := createRoutingTable(t, id)
	defer cleanup()
	now := time.Now().UTC()

	err := rt.createOrUpdateKBucket(id, now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(idStr, nil)
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
	now = time.Now().UTC()
	err = rt.SetBucketTimestamp(idStr, now)
	assert.NoError(t, err)
	ti, err = rt.GetBucketTimestamp(idStr, nil)
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}

func TestGetBucketTimestamp(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	now := time.Now().UTC()
	id := "AA"
	err := rt.createOrUpdateKBucket([]byte(id), now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(id, nil)
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}

func TestGetNodeRoutingTable(t *testing.T) {
	//TODO
}
