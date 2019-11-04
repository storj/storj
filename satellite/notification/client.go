// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"

	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

type client struct {
	conn   *rpc.Conn
	client rpc.NotificationClient
}

// newClient dials the target's endpoint
func newClient(ctx context.Context, dialer rpc.Dialer, address string, id storj.NodeID) (_ *client, err error) {
	var conn *rpc.Conn

	if !id.IsZero() {
		conn, err = dialer.DialAddressID(ctx, address, id)
		if err != nil {
			return nil, err
		}
	} else {
		conn, err = dialer.DialAddressInsecureBestEffort(ctx, address)
		if err != nil {
			return nil, err
		}
	}

	return &client{
		conn:   conn,
		client: conn.NotificationClient(),
	}, nil
}

// Close closes the connection
func (client *client) Close() error {
	return client.conn.Close()
}
