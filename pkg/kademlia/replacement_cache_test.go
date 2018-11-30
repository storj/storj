// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
)

func TestAddToReplacementCache(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes([]byte{244, 255}))
	defer cleanup()
	kadBucketID := keyToBucketID(teststorj.NodeIDFromBytes([]byte{255, 255}).Bytes())
	node1 := teststorj.MockNode(string([]byte{233, 255}))
	rt.addToReplacementCache(kadBucketID, node1)
	assert.Equal(t, []*pb.Node{node1}, rt.replacementCache[kadBucketID])
	kadBucketID2 := keyToBucketID(teststorj.NodeIDFromBytes([]byte{127, 255}).Bytes())
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
