// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

//NodeClientErr is the class for all errors pertaining to node client operations
var NodeClientErr = errs.Class("node client error")

// NewNodeClient instantiates a node client
func NewNodeClient(identity *provider.FullIdentity, self pb.Node, dht dht.DHT) (Client, error) {
	node := &Node{
		dht:  dht,
		self: self,
		pool: NewConnectionPool(identity),
	}

	node.pool.Init()

	return node, nil
}

// Client is the Node client communication interface
type Client interface {
	Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error)
	Ping(ctx context.Context, to pb.Node) (bool, error)
	Disconnect() error
}
