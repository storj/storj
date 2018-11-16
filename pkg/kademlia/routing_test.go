// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

var (
	testIDA = teststorj.NodeIDFromString("AA")
	testIDB = teststorj.NodeIDFromString("BB")
	testIDC = teststorj.NodeIDFromString("CC")
)

func TestLocal(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	assert.Equal(t, rt.Local().Id, "AA")
}

func TestK(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	k := rt.K()
	assert.Equal(t, rt.bucketSize, k)

}

func TestCacheSize(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	expected := rt.rcBucketSize
	result := rt.CacheSize()
	assert.Equal(t, expected, result)
}

func TestGetBucket(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	node := newNodeFromID(testIDA)
	node2 := newNodeFromID(testIDB)
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)

	cases := []struct {
		nodeID   storj.NodeID
		expected *KBucket
		ok       bool
	}{
		{nodeID: node.Id,
			expected: &KBucket{nodes: []storj.Node{node, node2}},
			ok:       true,
		},
		{nodeID: node2.Id,
			expected: &KBucket{nodes: []storj.Node{node, node2}},
			ok:       true,
		},
	}
	for i, v := range cases {
		assert.NoError(t, err)
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
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	node := newNodeFromID(testIDA)
	node2 := newNodeFromID(testIDB)
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)
	expected := []storj.Node{node, node2}
	buckets, err := rt.GetBuckets()
	assert.NoError(t, err)
	for _, v := range buckets {
		for j, w := range v.Nodes() {
			assert.True(t, proto.Equal(expected[j], w))
		}
	}
}

func TestFindNear(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	node1 := newNodeFromID(testIDA)
	node2 := newNodeFromID(testIDB)
	node3 := newNodeFromID(testIDC)
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)

	cases := []struct {
		testID        string
		node          storj.Node
		expectedNodes []storj.Node
		limit         int
	}{
		{testID: "limit 1 on node1: return node1",
			node:          node1,
			expectedNodes: []storj.Node{node1},
			limit:         1,
		},
		{testID: "limit 2 on node3: return nodes2, node1",
			node:          node3,
			expectedNodes: []storj.Node{node2, node1},
			limit:         2,
		},
		{testID: "limit 1 on node3: return node2",
			node:          node3,
			expectedNodes: []storj.Node{node2},
			limit:         1,
		},
		{testID: "limit 3 on node3: return node2, node1",
			node:          node3,
			expectedNodes: []storj.Node{node2, node1},
			limit:         3,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ns, err := rt.FindNear(c.node.Id, c.limit)
			assert.NoError(t, err)
			assert.Equal(t, c.expectedNodes, ns)
		})
	}
}

func TestConnectionSuccess(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	
	address1 := &pb.NodeAddress{Address: "a"}
	address2 := &pb.NodeAddress{Address: "b"}
	node1 := storj.NewNodeWithID(testIDA, &pb.Node{Address: address1})
	node2 := storj.NewNodeWithID(testIDB, &pb.Node{Address: address2})
	cases := []struct {
		testID  string
		node    storj.Node
		id      storj.NodeID
		address *pb.NodeAddress
	}{
		{testID: "Update Node",
			node:    node1,
			id:      testIDA,
			address: address1,
		},
		{testID: "Add Node",
			node:    node2,
			id:      testIDB,
			address: address2,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			err := rt.ConnectionSuccess(c.node)
			assert.NoError(t, err)
			v, err := rt.nodeBucketDB.Get(c.id.Bytes())
			assert.NoError(t, err)
			n, err := unmarshalNodes(storage.Keys{storage.Key(c.id.Bytes())}, []storage.Value{v})
			assert.NoError(t, err)
			assert.Equal(t, c.address.Address, n[0].Address.Address)
		})
	}
}

func TestConnectionFailed(t *testing.T) {
	node := newNodeFromID(testIDA)
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	err := rt.ConnectionFailed(node)
	assert.NoError(t, err)
	v, err := rt.nodeBucketDB.Get(testIDA.Bytes())
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestSetBucketTimestamp(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	now := time.Now().UTC()

	err := rt.createOrUpdateKBucket(testIDA.Bytes(), now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(testIDA, nil)
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
	now = time.Now().UTC()
	err = rt.SetBucketTimestamp(testIDA, now)
	assert.NoError(t, err)
	ti, err = rt.GetBucketTimestamp(testIDA, nil)
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}

func TestGetBucketTimestamp(t *testing.T) {
	rt, cleanup := createRoutingTable(t, testIDA)
	defer cleanup()
	now := time.Now().UTC()
	err := rt.createOrUpdateKBucket(testIDA.Bytes(), now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(testIDA, nil)
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}
