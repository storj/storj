// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

func (rt *RoutingTable) updateReplacementCache(kadBucketID storage.Key, nodes []*proto.Node) {
	bucketID := string(kadBucketID)
	rt.replacementCache[bucketID] = nodes
}

func (rt *RoutingTable) getReplacementCacheBucket(kadBucketID storage.Key) []*proto.Node {
	return rt.replacementCache[string(kadBucketID)]
}

func (rt *RoutingTable) addToReplacementCache(kadBucketID storage.Key, node *proto.Node) {
	bucketID := string(kadBucketID)
	nodes := rt.replacementCache[bucketID]
	nodes = append(nodes, node)
	rt.replacementCache[bucketID] = nodes
}
