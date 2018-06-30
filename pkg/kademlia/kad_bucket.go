// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"storj.io/storj/storage"
	proto "storj.io/storj/protos/overlay"
)

//Have yet to use this implementation of the Bucket interface - JJ


// KadBucket implements the Bucket interface
type KadBucket struct {
	id storage.Key
	leftEndpoint storage.Key
	nodes *storage.Keys
}

//NewKadBucket returns a newly configured instance of a KadBucket
func NewKadBucket(id string, nodes *storage.Keys) KadBucket {
	//WIP
	return KadBucket{
	}
}

// Routing ... TODO
func (b *KadBucket) Routing() []proto.Node {
	return []proto.Node{}
}

// Cache ... TODO
func (b *KadBucket) Cache() []proto.Node {
	return []proto.Node{}
}

// Midpoint returns the midpoint of the bucket
func (b *KadBucket) Midpoint() string {
	// left := []byte(b.leftEndpoint)
	// right := []byte(b.id)

	return ""
}

// Nodes returns the set of all nodes in a bucket
func (b *KadBucket) Nodes()[]*proto.Node {
	//return *(b.nodes)
	return []*proto.Node{}
}

