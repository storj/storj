// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"time"

	"storj.io/storj/protos/overlay"
)

// RouteTable implements the RoutingTable interface
type RouteTable struct {
}

// LocalID returns the local nodes ID
func (rt RouteTable) LocalID() NodeID {
	return ""
}

// K returns the currently configured maximum of nodes to store in a bucket
func (rt RouteTable) K() int {
	return 0
}

// CacheSize returns the total current size of the cache
func (rt RouteTable) CacheSize() int {
	return 0
}

// GetBucket retrieves a bucket from the local node
func (rt RouteTable) GetBucket(id string) (bucket Bucket, ok bool) {
	return KBucket{}, true
}

// GetBuckets retrieves all buckets from the local node
func (rt RouteTable) GetBuckets() ([]*Bucket, error) {
	return []*Bucket{}, nil
}

// FindNear finds all Nodes near the provided nodeID up to the provided limit
func (rt RouteTable) FindNear(id NodeID, limit int) ([]overlay.Node, error) {
	return []overlay.Node{}, nil
}

// ConnectionSuccess handles the details of what kademlia should do when
// a successful connection is made to node on the network
func (rt RouteTable) ConnectionSuccess(id string, address overlay.NodeAddress) {
	return
}

// ConnectionFailed handles the details of what kademlia should do when
// a connection fails for a node on the network
func (rt RouteTable) ConnectionFailed(id string, address overlay.NodeAddress) {
	return
}

// SetBucketTimestamp updates the last updated time for a bucket
func (rt RouteTable) SetBucketTimestamp(id string, now time.Time) error {
	return nil
}

// GetBucketTimestamp retrieves the last updated time for a bucket
func (rt RouteTable) GetBucketTimestamp(id string, bucket Bucket) (time.Time, error) {
	return time.Now(), nil
}
