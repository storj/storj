// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build drpc

package rpcpeer

import (
	"context"
	"crypto/tls"

	"storj.io/drpc/drpcctx"
)

// internalFromContext returns a peer from the context using drpc.
func internalFromContext(ctx context.Context) (*Peer, error) {
	tr, ok := drpcctx.Transport(ctx)
	if !ok {
		return nil, Error.New("unable to get drpc peer from context")
	}

	conn, ok := tr.(*tls.Conn)
	if !ok {
		return nil, Error.New("drpc transport is not a *tls.Conn")
	}

	return &Peer{
		Addr:  conn.RemoteAddr(),
		State: conn.ConnectionState(),
	}, nil
}
