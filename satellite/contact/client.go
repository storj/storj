// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

type client struct {
	conn   *grpc.ClientConn
	client pb.ContactClient
}

// newClient dials the target contact endpoint
func newClient(ctx context.Context, transport transport.Client, target *pb.NodeAddress, peerIDFromContext storj.NodeID) (*client, error) {
	opts, err := tlsopts.NewOptions(transport.Identity(), tlsopts.Config{PeerIDVersions: "latest"}, nil)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dialOption, err := opts.DialOption(peerIDFromContext)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	conn, err := transport.DialAddress(ctx, target.Address, dialOption)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &client{
		conn:   conn,
		client: pb.NewContactClient(conn),
	}, nil
}

// pingNode pings a node
func (client *client) pingNode(ctx context.Context, req *pb.ContactPingRequest, opt grpc.CallOption) (*pb.ContactPingResponse, error) {
	return client.client.PingNode(ctx, req, opt)
}

// close closes the connection
func (client *client) close() error {
	return client.conn.Close()
}
