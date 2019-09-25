// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !drpc

package rpcpeer

import (
	"context"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// internalFromContext returns a peer from the context using grpc.
func internalFromContext(ctx context.Context) (*Peer, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, Error.New("unable to get grpc peer from context")
	}

	tlsInfo, ok := peer.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, Error.New("peer AuthInfo is not credentials.TLSInfo")
	}

	return &Peer{
		Addr:  peer.Addr,
		State: tlsInfo.State,
	}, nil
}
