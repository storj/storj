// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

type client struct {
	conn   *rpc.Conn
	client rpc.ContactClient
}

// newClient dials the target contact endpoint
func newClient(ctx context.Context, dialer rpc.Dialer, address string, id storj.NodeID) (*client, error) {
	conn, err := dialer.DialAddressID(ctx, address, id)
	if err != nil {
		return nil, err
	}

	return &client{
		conn:   conn,
		client: conn.ContactClient(),
	}, nil
}

// pingNode pings a node
func (client *client) pingNode(ctx context.Context, req *pb.ContactPingRequest) (*pb.ContactPingResponse, error) {
	return client.client.PingNode(ctx, req)
}

// Close closes the connection
func (client *client) Close() error {
	return client.conn.Close()
}
