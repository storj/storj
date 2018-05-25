// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"strconv"
	"time"

	bkad "github.com/coyle/kademlia"

	"storj.io/storj/protos/overlay"
)

// RouteTable implements the RoutingTable interface
type RouteTable struct {
	ht  *bkad.HashTable
	dht *bkad.DHT
}

// LocalID returns the local nodes ID
func (rt RouteTable) LocalID() NodeID {
	return NodeID(rt.dht.GetSelfID())
}

// K returns the currently configured maximum of nodes to store in a bucket
func (rt RouteTable) K() int {
	return rt.dht.NumNodes()
}

// CacheSize returns the total current size of the cache
func (rt RouteTable) CacheSize() int {
	//TODO: How is this calculated ? size of the routing table ? is it total bytes, mb, kb etc .?
	return 0
}

// GetBucket retrieves a bucket from the local node
func (rt RouteTable) GetBucket(id string) (bucket Bucket, ok bool) {
	i, err := strconv.Atoi(id)
	if err != nil {
		return KBucket{}, false
	}
	b := rt.ht.GetBucket(i)
	if b == nil {
		return KBucket{}, false
	}

	return KBucket{
		nodes: convertNetworkNodes(b),
	}, true
}

// GetBuckets retrieves all buckets from the local node
func (rt RouteTable) GetBuckets() (k []Bucket, err error) {
	bs := []Bucket{}
	b := rt.ht.GetBuckets()
	for i, v := range b {
		bs[i] = KBucket{nodes: convertNetworkNodes(v)}
	}

	return bs, nil
}

// FindNear finds all Nodes near the provided nodeID up to the provided limit
func (rt RouteTable) FindNear(id NodeID, limit int) ([]overlay.Node, error) {
	return convertNetworkNodes(rt.ht.GetClosestContacts([]byte(id), limit)), nil
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
