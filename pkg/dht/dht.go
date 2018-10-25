// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dht

import (
	"context"
	"time"

	"storj.io/storj/pkg/pb"
)

// NodeID is the unique identifier used for Nodes in the DHT
type NodeID interface {
	String() string
	Bytes() []byte
}

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	GetNodes(ctx context.Context, start string, limit int, restrictions ...pb.Restriction) ([]*pb.Node, error)
	GetRoutingTable(ctx context.Context) (RoutingTable, error)
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node pb.Node) (pb.Node, error)
	FindNode(ctx context.Context, ID NodeID) (pb.Node, error)
	Disconnect() error
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	// local params
	Local() pb.Node
	K() int
	CacheSize() int

	GetBucket(id string) (bucket Bucket, ok bool)
	GetBuckets() ([]Bucket, error)

	FindNear(id NodeID, limit int) ([]*pb.Node, error)

	ConnectionSuccess(node *pb.Node) error
	ConnectionFailed(node *pb.Node) error

	// these are for refreshing
	SetBucketTimestamp(id string, now time.Time) error
	GetBucketTimestamp(id string, bucket Bucket) (time.Time, error)
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	Routing() []pb.Node
	Cache() []pb.Node
	Midpoint() string
	Nodes() []*pb.Node
}
