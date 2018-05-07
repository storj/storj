package kademlia

import (
	"context"

	"storj.io/storj/protos/overlay"
)

type NodeID string

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	GetNodes(ctx context.Context) []overlay.Node
	GetRoutingTable(ctx context.Context) RoutingTable
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node overlay.Node) overlay.Node
	FindNode(ctx context.Context, ID NodeID) overlay.Node
	FindValue(ctx context.Context, ID NodeID) overlay.Node
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	GetBuckets() []*Bucket
	GetBucket(id string) (ok bool, bucket Bucket)
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
