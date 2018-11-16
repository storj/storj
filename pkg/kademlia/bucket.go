// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"storj.io/storj/pkg/storj"
)

// KBucket implements the Bucket interface
type KBucket struct {
	nodes []storj.Node
}

// Routing __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Routing() []storj.Node {
	return []storj.Node{}
}

// Cache __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Cache() []storj.Node {
	return []storj.Node{}
}

// Midpoint __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b *KBucket) Midpoint() string {
	return ""
}

// Nodes returns the set of all nodes in a bucket
func (b *KBucket) Nodes() []storj.Node {
	return b.nodes
}
