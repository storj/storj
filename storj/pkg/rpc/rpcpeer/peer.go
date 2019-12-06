// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcpeer

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/zeebo/errs"
)

// Error is the class of errors returned by this package.
var Error = errs.Class("rpcpeer")

// Peer represents an rpc peer.
type Peer struct {
	Addr  net.Addr
	State tls.ConnectionState
}

// peerKey is used as a unique value for context keys.
type peerKey struct{}

// NewContext returns a new context with the peer associated as a value.
func NewContext(ctx context.Context, peer *Peer) context.Context {
	return context.WithValue(ctx, peerKey{}, peer)
}

// FromContext returns the peer that was previously associated by NewContext.
func FromContext(ctx context.Context) (*Peer, error) {
	if peer, ok := ctx.Value(peerKey{}).(*Peer); ok {
		return peer, nil
	} else if peer, drpcErr := drpcInternalFromContext(ctx); drpcErr == nil {
		return peer, nil
	} else if peer, grpcErr := grpcInternalFromContext(ctx); grpcErr == nil {
		return peer, nil
	} else {
		return nil, errs.Combine(drpcErr, grpcErr)
	}
}
