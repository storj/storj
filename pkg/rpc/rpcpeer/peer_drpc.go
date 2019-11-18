// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcpeer

import (
	"context"
	"crypto/tls"
	"net"

	"storj.io/drpc/drpcctx"
)

// drpcInternalFromContext returns a peer from the context using drpc.
func drpcInternalFromContext(ctx context.Context) (*Peer, error) {
	tr, ok := drpcctx.Transport(ctx)
	if !ok {
		return nil, Error.New("unable to get drpc peer from context")
	}

	conn, ok := tr.(interface {
		RemoteAddr() net.Addr
		ConnectionState() tls.ConnectionState
	})
	if !ok {
		return nil, Error.New("drpc transport does not have required methods")
	}

	return &Peer{
		Addr:  conn.RemoteAddr(),
		State: conn.ConnectionState(),
	}, nil
}
