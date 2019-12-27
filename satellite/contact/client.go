// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
)

type client struct {
	conn   *rpc.Conn
	client pb.DRPCContactClient
}

// newClient dials the target contact endpoint
func newClient(ctx context.Context, dialer rpc.Dialer, address string, id storj.NodeID) (*client, error) {
	conn, err := dialer.DialAddressID(ctx, address, id)
	if err != nil {
		return nil, err
	}

	return &client{
		conn:   conn,
		client: pb.NewDRPCContactClient(conn.Raw()),
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
