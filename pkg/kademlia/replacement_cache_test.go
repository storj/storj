// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"
)

func TestAddToReplacementCache(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes([]byte{244, 255}))
	defer cleanup()

	kadBucketID := []byte{255, 255}
	node1 := newNodeFromID(teststorj.NodeIDFromBytes([]byte{233, 255}))
	rt.addToReplacementCache(kadBucketID, node1)
	assert.Equal(t, []storj.Node{node1}, rt.replacementCache[string(kadBucketID)])
	kadBucketID2 := []byte{127, 255}
	node2 := newNodeFromID(teststorj.NodeIDFromBytes([]byte{100, 255}))
	node3 := newNodeFromID(teststorj.NodeIDFromBytes([]byte{90, 255}))
	node4 := newNodeFromID(teststorj.NodeIDFromBytes([]byte{80, 255}))
	rt.addToReplacementCache(kadBucketID2, node2)
	rt.addToReplacementCache(kadBucketID2, node3)

	assert.Equal(t, []storj.Node{node1}, rt.replacementCache[string(kadBucketID)])
	assert.Equal(t, []storj.Node{node2, node3}, rt.replacementCache[string(kadBucketID2)])
	rt.addToReplacementCache(kadBucketID2, node4)
	assert.Equal(t, []storj.Node{node3, node4}, rt.replacementCache[string(kadBucketID2)])
}
