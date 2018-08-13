// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dht

import (
	"context"
	"time"

	proto "storj.io/storj/protos/overlay"
)

// NodeID is the unique identifer used for Nodes in the DHT
type NodeID interface {
	String() string
	Bytes() []byte
}

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	GetNodes(ctx context.Context, start string, limit int, restrictions ...proto.Restriction) ([]*proto.Node, error)
	GetRoutingTable(ctx context.Context) (RoutingTable, error)
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node proto.Node) (proto.Node, error)
	FindNode(ctx context.Context, ID NodeID) (proto.Node, error)
	Disconnect() error
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	// local params
	Local() proto.Node
	K() int
	CacheSize() int

	GetBucket(id string) (bucket Bucket, ok bool)
	GetBuckets() ([]Bucket, error)

	FindNear(id NodeID, limit int) ([]*proto.Node, error)

	ConnectionSuccess(id string, address proto.NodeAddress)
	ConnectionFailed(id string, address proto.NodeAddress)

	// these are for refreshing
	SetBucketTimestamp(id string, now time.Time) error
	GetBucketTimestamp(id string, bucket Bucket) (time.Time, error)
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	Routing() []proto.Node
	Cache() []proto.Node
	Midpoint() string
	Nodes() []*proto.Node
}
