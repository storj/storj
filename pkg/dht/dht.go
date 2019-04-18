// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dht

import (
	"context"
	"time"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	FindNear(ctx context.Context, start storj.NodeID, limit int) ([]*pb.Node, error)
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node pb.Node) (pb.Node, error)
	FindNode(ctx context.Context, ID storj.NodeID) (pb.Node, error)
	Seen() []*pb.Node
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	// local params
	Local() *overlay.NodeDossier
	K() int
	CacheSize() int
	GetBucketIds() (storage.Keys, error)
	FindNear(id storj.NodeID, limit int) ([]*pb.Node, error)
	ConnectionSuccess(node *pb.Node) error
	ConnectionFailed(node *pb.Node) error
	// these are for refreshing
	SetBucketTimestamp(id []byte, now time.Time) error
	GetBucketTimestamp(id []byte) (time.Time, error)

	Close() error
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	Routing() []pb.Node
	Cache() []pb.Node
	// TODO: should this be a NodeID?
	Midpoint() string
	Nodes() []*pb.Node
}
