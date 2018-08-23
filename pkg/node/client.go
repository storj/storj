// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package node

import (
	"context"

	"storj.io/storj/pkg/pool"

	"storj.io/storj/pkg/transport"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/pkg/provider"
)

// NewNodeClient instantiates a node client
func NewNodeClient(self proto.Node) (Client, error) {
	ca, err := provider.NewCA(context.Background(), 12, 4)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}

	client := transport.NewClient(identity)
	return &Node{
		self:  self,
		tc:    client,
		cache: pool.NewConnectionPool(),
	}, nil
}

// Client is the Node client communication interface
type Client interface {
	Lookup(ctx context.Context, to proto.Node, find proto.Node) ([]*proto.Node, error)
}
