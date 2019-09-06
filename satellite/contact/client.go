// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

type client struct {
	log    *zap.Logger
	conn   *grpc.ClientConn
	client pb.ContactClient
}

// newClient dials the target contact endpoint
func newClient(ctx context.Context, log *zap.Logger, transport transport.Client, target *pb.NodeAddress) (*client, error) {
	conn, err := transport.DialAddress(ctx, target.Address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &client{
		log:    log,
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
