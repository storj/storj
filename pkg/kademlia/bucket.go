// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import "storj.io/storj/pkg/pb"

// KBucket implements the Bucket interface
type KBucket struct {
	nodes []*pb.Node
}

// Routing __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Routing() []pb.Node {
	return []pb.Node{}
}

// Cache __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Cache() []pb.Node {
	return []pb.Node{}
}

// Midpoint __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Midpoint() string {
	return ""
}

// Nodes returns the set of all nodes in a bucket
func (b *KBucket) Nodes() []*pb.Node {
	return b.nodes
}
