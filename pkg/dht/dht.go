// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dht

import (
	"context"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// NodeID is the unique identifier used for Nodes in the DHT
type NodeID interface {
	String() string
	Bytes() []byte
}

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	GetNodes(ctx context.Context, start storj.NodeID, limit int, restrictions ...pb.Restriction) ([]storj.Node, error)
	GetRoutingTable(ctx context.Context) (RoutingTable, error)
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node storj.Node) (storj.Node, error)
	FindNode(ctx context.Context, ID storj.NodeID) (storj.Node, error)
	Disconnect() error
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	// local params
	Local() storj.Node
	K() int
	CacheSize() int

	GetBucket(id storj.NodeID) (bucket Bucket, ok bool)
	GetBuckets() ([]Bucket, error)

	FindNear(id storj.NodeID, limit int) ([]storj.Node, error)

	ConnectionSuccess(node storj.Node) error
	ConnectionFailed(node storj.Node) error

	// these are for refreshing
	SetBucketTimestamp(id storj.NodeID, now time.Time) error
	GetBucketTimestamp(id storj.NodeID, bucket Bucket) (time.Time, error)
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	Routing() []storj.Node
	Cache() []storj.Node
	Midpoint() string
	Nodes() []storj.Node
}
