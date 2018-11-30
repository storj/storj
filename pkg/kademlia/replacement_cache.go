// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"storj.io/storj/pkg/pb"
)

func (rt *RoutingTable) addToReplacementCache(kadBucketID bucketID, node *pb.Node) {
	nodes := rt.replacementCache[kadBucketID]
	nodes = append(nodes, node)
	if len(nodes) > rt.rcBucketSize {
		copy(nodes, nodes[1:])
		nodes = nodes[:len(nodes)-1]
	}
	rt.replacementCache[kadBucketID] = nodes
}
