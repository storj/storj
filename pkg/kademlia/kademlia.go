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
	GetNodes(ctx context.Context, limit, offset int) []overlay.Node
	GetRoutingTable(ctx context.Context) RoutingTable
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node overlay.Node) overlay.Node
	FindNode(ctx context.Context, ID NodeID) overlay.Node
	FindValue(ctx context.Context, ID NodeID) overlay.Node
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	// local params
	LocalID() NodeID
	K() int
	CacheSize() int

	GetBucket(id string) (bucket Bucket, ok bool)
	GetBuckets(cb func(Bucket) (keep_going bool))

	FindNear(id NodeID, limit, offset int) ([]overlay.Node, error)

	ConnectionSuccess(id string, address overlay.NodeAddress)
	ConnectionFailed(id string, address overlay.NodeAddress)

	// these are for refreshing
	SetBucketTimestamp(id string, now time.Time) error
	GetBucketTimestamp(id string, bucket Bucket) (time.Time, error)
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	MoveToTail(ID NodeID) error
	MoveToHead(ID NodeID) error
	Has(ID NodeID) bool
	Add(ID NodeID) error
	Remove(ID NodeID) error
	Get(ID NodeID) overlay.Node
	Len() int
}
