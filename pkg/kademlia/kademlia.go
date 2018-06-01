// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"time"

	"storj.io/storj/protos/overlay"
)

// NodeID is the unique identifer for a node on the network
type NodeID string

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	GetNodes(ctx context.Context, start string, limit int) ([]overlay.Node, error)

	GetRoutingTable(ctx context.Context) (RoutingTable, error)
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node overlay.Node) (overlay.Node, error)
	FindNode(ctx context.Context, ID NodeID) (overlay.Node, error)
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	// local params
	LocalID() NodeID
	K() int
	CacheSize() int

	GetBucket(id string) (bucket Bucket, ok bool)
	GetBuckets() ([]Bucket, error)

	FindNear(id NodeID, limit int) ([]overlay.Node, error)

	ConnectionSuccess(id string, address overlay.NodeAddress)
	ConnectionFailed(id string, address overlay.NodeAddress)

	// these are for refreshing
	SetBucketTimestamp(id string, now time.Time) error
	GetBucketTimestamp(id string, bucket Bucket) (time.Time, error)
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	Routing() []overlay.Node
	Cache() []overlay.Node
	Midpoint() string
}
