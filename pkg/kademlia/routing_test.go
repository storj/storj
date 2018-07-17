// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/dht"
	proto "storj.io/storj/protos/overlay"
)

func TestLocal(t *testing.T) {
	rt := createRT()
	local := rt.Local()
	assert.Equal(t, *rt.self, local)
}

func TestK(t *testing.T) {
	rt := createRT()
	k := rt.K()
	assert.Equal(t, rt.bucketSize, k)

}

func TestCacheSize(t *testing.T) {
	//TODO
	rt := createRT()
	expected := 0
	result := rt.CacheSize()
	assert.Equal(t, expected, result)
}

func TestGetBucket(t *testing.T) {
	rt := createRT()
	rt.self.Id = "AA"
	node := mockNode(rt.self.Id)
	node2 := mockNode("BB")
	err := rt.addNode(node)
	assert.NoError(t, err)
	err = rt.addNode(node2)
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
		assert.Equal(t, v.expected, b)
		assert.Equal(t, v.ok, e)
		fmt.Printf("error occured at index %d", i)
	}
}

func TestGetBuckets(t *testing.T) {
	rt := createRT()
	rt.self.Id = "AA"
	node := mockNode(rt.self.Id)
	node2 := mockNode("BB")
	err := rt.addNode(node)
	assert.NoError(t, err)
	err = rt.addNode(node2)
	assert.NoError(t, err)
	expected := []dht.Bucket{&KBucket{nodes: []*proto.Node{node, node2}}}
	buckets, err := rt.GetBuckets()
	assert.NoError(t, err)
	assert.Equal(t, expected, buckets)
}

func TestFindNear(t *testing.T) {
	rt := createRT()
	rt.self.Id = "AA"
	node := mockNode(rt.self.Id)
	node2 := mockNode("BB")
	err := rt.addNode(node)
	assert.NoError(t, err)
	err = rt.addNode(node2)
	assert.NoError(t, err)
	expected := []*proto.Node{node2}
	nodes, err := rt.FindNear(StringToNodeID(node.Id), 1)
	assert.NoError(t, err)
	assert.Equal(t, expected, nodes)
}

func TestConnectionSuccess(t *testing.T) {
	//TODO
}

func TestConnectionFailed(t *testing.T) {
	//TODO
}

func TestSetBucketTimestamp(t *testing.T) {
	rt := createRT()
	now := time.Now().UTC()
	id := "AA"
	err := rt.createOrUpdateKBucket([]byte(id), now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(id, &KBucket{})
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
	now = time.Now().UTC()
	err = rt.SetBucketTimestamp(id, now)
	assert.NoError(t, err)
	ti, err = rt.GetBucketTimestamp(id, &KBucket{})
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}

func TestGetBucketTimestamp(t *testing.T) {
	rt := createRT()
	now := time.Now().UTC()
	id := "AA"
	err := rt.createOrUpdateKBucket([]byte(id), now)
	assert.NoError(t, err)
	ti, err := rt.GetBucketTimestamp(id, &KBucket{})
	assert.Equal(t, now, ti)
	assert.NoError(t, err)
}

func TestGetNodeRoutingTable(t *testing.T) {
	//TODO
}
