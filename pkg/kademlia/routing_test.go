// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"fmt"
	"testing"
	"time"

	pb "github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	proto "storj.io/storj/protos/overlay"
)

func TestLocal(t *testing.T) {
	rt := createRT([]byte("AA"))
	local := rt.Local()
	assert.Equal(t, *rt.self, local)
}

func TestK(t *testing.T) {
	rt := createRT([]byte("AA"))
	k := rt.K()
	assert.Equal(t, rt.bucketSize, k)

}

func TestCacheSize(t *testing.T) {
	//TODO
	rt := createRT([]byte("AA"))
	expected := 0
	result := rt.CacheSize()
	assert.Equal(t, expected, result)
}

func TestGetBucket(t *testing.T) {
	rt := createRT([]byte("AA"))
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
			assert.True(t, pb.Equal(w, b.Nodes()[j]))
		}
		assert.Equal(t, v.ok, e)
		fmt.Printf("error occured at index %d", i) //what's a better way to print the index?
	}
}

func TestGetBuckets(t *testing.T) {
	rt := createRT([]byte("AA"))
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
	rt := createRT([]byte("AA"))
	node := mockNode("AA")
	node2 := mockNode("BB")
	ok, err := rt.addNode(node2)
	assert.True(t, ok)
	assert.NoError(t, err)
	expected := []*proto.Node{node2}
	nodes, err := rt.FindNear(StringToNodeID(node.Id), 1)
	assert.NoError(t, err)
	for i, v := range nodes {
		assert.True(t, pb.Equal(expected[i], v))
	}
}

func TestConnectionSuccess(t *testing.T) {
	//TODO
}

func TestConnectionFailed(t *testing.T) {
	//TODO
}

func TestSetBucketTimestamp(t *testing.T) {
	id := []byte("AA")
	idStr := string(id)
	rt := createRT(id)
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
	rt := createRT([]byte("AA"))
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
