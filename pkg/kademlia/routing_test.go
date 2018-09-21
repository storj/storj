// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
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
			expected: &KBucket{nodes: []*pb.Node{node, node2}},
			ok:       true,
		},
		{nodeID: node2.Id,
			expected: &KBucket{nodes: []*pb.Node{node, node2}},
			ok:       true,
		},
	}
	for i, v := range cases {
		b, e := rt.GetBucket(node2.Id)
		for j, w := range v.expected.nodes {
			if !assert.True(t, proto.Equal(w, b.Nodes()[j])) {
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
	expected := []*pb.Node{node, node2}
	buckets, err := rt.GetBuckets()
	assert.NoError(t, err)
	for _, v := range buckets {
		for j, w := range v.Nodes() {
			assert.True(t, proto.Equal(expected[j], w))
		}
	}
}

func TestFindNear(t *testing.T) {
	rt, cleanup := createRoutingTable(t, []byte("AA"))
	defer cleanup()
	node1 := mockNode("AA")
	node2 := mockNode("BB")
	node3 := mockNode("CC")
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)

	cases := []struct {
		testID        string
		node          pb.Node
		expectedNodes []*pb.Node
		limit         int
	}{
		{testID: "limit 1 on node1: return node1",
			node:          *node1,
			expectedNodes: []*pb.Node{node1},
			limit:         1,
		},
		{testID: "limit 2 on node3: return nodes2, node1",
			node:          *node3,
			expectedNodes: []*pb.Node{node2, node1},
			limit:         2,
		},
		{testID: "limit 1 on node3: return node2",
			node:          *node3,
			expectedNodes: []*pb.Node{node2},
			limit:         1,
		},
		{testID: "limit 3 on node3: return node2, node1",
			node:          *node3,
			expectedNodes: []*pb.Node{node2, node1},
			limit:         3,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ns, err := rt.FindNear(node.IDFromString(c.node.Id), c.limit)
			assert.NoError(t, err)
			assert.Equal(t, c.expectedNodes, ns)
		})
	}
}

func TestConnectionSuccess(t *testing.T) {
	id := "AA"
	rt, cleanup := createRoutingTable(t, []byte(id))
	defer cleanup()
	id2 := "BB"
	address1 := &pb.NodeAddress{Address: "a"}
	address2 := &pb.NodeAddress{Address: "b"}
	node1 := &pb.Node{Id: id, Address: address1}
	node2 := &pb.Node{Id: id2, Address: address2}
	cases := []struct {
		testID  string
		node    *pb.Node
		id      string
		address *pb.NodeAddress
	}{
		{testID: "Update Node",
			node:    node1,
			id:      id,
			address: address1,
		},
		{testID: "Add Node",
			node:    node2,
			id:      id2,
			address: address2,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			err := rt.ConnectionSuccess(c.node)
			assert.NoError(t, err)
			v, err := rt.nodeBucketDB.Get([]byte(c.id))
			assert.NoError(t, err)
			n, err := unmarshalNodes(storage.Keys{storage.Key(c.id)}, []storage.Value{v})
			assert.NoError(t, err)
			assert.Equal(t, c.address.Address, n[0].Address.Address)
		})
	}
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
