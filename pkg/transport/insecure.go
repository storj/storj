// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"

	"google.golang.org/grpc"
)

// DialAddressInsecure returns an insecure grpc connection without tls to a node.
//
// Use this method for communication with localhost. For example, with the inspector or debugging services.
// Otherwise in most cases DialNode should be used for communicating with nodes since it is secure.
func DialAddressInsecure(ctx context.Context, address string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)
	options := append([]grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
	}, opts...)

	timedCtx, cf := context.WithTimeout(ctx, defaultTransportDialTimeout)
	defer cf()

	conn, err = grpc.DialContext(timedCtx, address, options...)
	if err == context.Canceled {
		return nil, err
	}
	return conn, Error.Wrap(err)
}
