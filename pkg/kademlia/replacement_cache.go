// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)


func (rt *RoutingTable) addToReplacementCache(kadBucketID storage.Key, node *proto.Node) {
	//get length of nodes
	//if length of nodes is equal to rt.replacementCacheSize, pop node off bottom of stack
	
	bucketID := string(kadBucketID)
	nodes := rt.replacementCache[bucketID]
	nodes = append(nodes, node)
	rt.replacementCache[bucketID] = nodes
}
