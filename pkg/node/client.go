// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

//NodeClientErr is the class for all errors pertaining to node client operations
var NodeClientErr = errs.Class("node client error")

// NewNodeClient instantiates a node client
func NewNodeClient(identity *provider.FullIdentity, self pb.Node, dht dht.DHT) (Client, error) {
	client := transport.NewClient(identity)
	return &Node{
		dht:   dht,
		self:  self,
		tc:    client,
		cache: NewConnectionPool(),
	}, nil
}

// Client is the Node client communication interface
type Client interface {
	Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error)
	Disconnect(ctx context.Context) error
}
