// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestAddToReplacementCache(t *testing.T) {
	rt, cleanup := createRoutingTable(t, storj.NodeID{244, 255})
	defer cleanup()

	kadBucketID := bucketID{255, 255}
	node1 := teststorj.MockNode(string([]byte{233, 255}))
	rt.addToReplacementCache(kadBucketID, node1)
	assert.Equal(t, []*pb.Node{node1}, rt.replacementCache[kadBucketID])
	kadBucketID2 := bucketID{127, 255}
	node2 := teststorj.MockNode(string([]byte{100, 255}))
	node3 := teststorj.MockNode(string([]byte{90, 255}))
	node4 := teststorj.MockNode(string([]byte{80, 255}))
	rt.addToReplacementCache(kadBucketID2, node2)
	rt.addToReplacementCache(kadBucketID2, node3)

	assert.Equal(t, []*pb.Node{node1}, rt.replacementCache[kadBucketID])
	assert.Equal(t, []*pb.Node{node2, node3}, rt.replacementCache[kadBucketID2])
	rt.addToReplacementCache(kadBucketID2, node4)
	assert.Equal(t, []*pb.Node{node3, node4}, rt.replacementCache[kadBucketID2])
}
