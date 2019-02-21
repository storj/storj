// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dht

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// DHT is the interface for the DHT in the Storj network
type DHT interface {
	FindNear(ctx context.Context, start storj.NodeID, limit int, restrictions ...pb.Restriction) ([]*pb.Node, error)
	Bootstrap(ctx context.Context) error
	Ping(ctx context.Context, node pb.Node) (pb.Node, error)
	FindNode(ctx context.Context, ID storj.NodeID) (pb.Node, error)
	Seen() []*pb.Node
}

// Bucket is a set of methods to act on kademlia k buckets
type Bucket interface {
	Routing() []pb.Node
	Cache() []pb.Node
	// TODO: should this be a NodeID?
	Midpoint() string
	Nodes() []*pb.Node
}
