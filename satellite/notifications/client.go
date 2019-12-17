// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"

	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

type Client struct {
	rpc.NotificationReceiverClient
	conn *rpc.Conn
}

func NewClient(ctx context.Context, dialer rpc.Dialer, nodeID storj.NodeID, address string) (*Client, error) {
	conn, err := dialer.DialAddressID(ctx, address, nodeID)
	if err != nil {
		return nil, err
	}

	return &Client{
		NotificationReceiverClient: conn.NotificationReceiverClient(),
		conn:                       conn,
	}, nil
}

func (client *Client) Close() error {
	return client.conn.Close()
}
