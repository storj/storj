// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

// Transport interface structure
type Transport struct {
	identity *provider.FullIdentity
}

// NewClient returns a newly instantiated Transport Client
func NewClient(identity *provider.FullIdentity) *Transport {
	return &Transport{identity: identity}
}

// DialNode using the authenticated mode
func (o *Transport) DialNode(ctx context.Context, node *pb.Node) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	if node.Address == nil || node.Address.Address == "" {
		return nil, Error.New("no address")
	}
	// TODO(coyle): pass ID
	dialOpt, err := o.identity.DialOption()
	if err != nil {
		return nil, err
	}
	return grpc.Dial(node.Address.Address, dialOpt)
}
