// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"
	proto "storj.io/storj/protos/overlay"
)

func TestAddToReplacementCache(t *testing.T) {
	rt := createRT([]byte{244, 255})
	kadBucketID := []byte{255, 255}
	node1 := mockNode(string([]byte{233, 255}))
	rt.addToReplacementCache(kadBucketID, node1)
	assert.Equal(t, []*proto.Node{node1}, rt.getReplacementCacheBucket(kadBucketID))
	kadBucketID2 := []byte{127, 255}
	node2 := mockNode(string([]byte{100, 255}))
	node3 := mockNode(string([]byte{90, 255}))
	rt.addToReplacementCache(kadBucketID2, node2)
	rt.addToReplacementCache(kadBucketID2, node3)
	assert.Equal(t, []*proto.Node{node1}, rt.getReplacementCacheBucket(kadBucketID))
	assert.Equal(t, []*proto.Node{node2, node3}, rt.getReplacementCacheBucket(kadBucketID2))
}
