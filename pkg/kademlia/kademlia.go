package kademlia

import (
	"context"

	"storj.io/storj/protos/overlay"
)

type nodeID string

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	GetNodes(ctx context.Context) []overlay.Node
	GetRoutingTable(ctx context.Context) RoutingTable
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node overlay.Node) overlay.Node
	FindNode(ctx context.Context, ID nodeID) overlay.Node
	FindValue(ctx context.Context)
}

// RoutingTable contains information on nodes we have locally
type RoutingTable interface {
	GetBuckets() []*Bucket
	GetBucket(id string) (ok bool, bucket Bucket)
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	MoveToTail(ID nodeID) error
	MoveToHead(ID nodeID) error
	Has(ID nodeID) bool
	Add(ID nodeID) error
	Remove(ID nodeID) error
	Get(ID nodeID) overlay.Node
	Len() int
}
