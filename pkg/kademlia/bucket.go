// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import "storj.io/storj/protos/overlay"

// KBucket implements the Bucket interface
type KBucket struct {
	nodes []*overlay.Node
}

// Routing __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b KBucket) Routing() []overlay.Node {
	return []overlay.Node{}
}

// Cache __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b KBucket) Cache() []overlay.Node {
	return []overlay.Node{}
}

// Midpoint __ (TODO) still not entirely sure what the bucket methods are supposed to do
func (b KBucket) Midpoint() string {
	return ""
}
