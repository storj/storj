// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import proto "storj.io/storj/protos/overlay"

// KBucket implements the Bucket interface
type KBucket struct {
	nodes []*proto.Node
}

// Routing __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Routing() []proto.Node {
	return []proto.Node{}
}

// Cache __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Cache() []proto.Node {
	return []proto.Node{}
}

// Midpoint __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Midpoint() string {
	return ""
}

// Nodes returns the set of all nodes in a bucket
func (b *KBucket) Nodes() []*proto.Node {
	return b.nodes
}
