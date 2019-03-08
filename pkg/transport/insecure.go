// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DialAddressEncrypted returns a grpc connection with encryption to a node; no client auth/cert is used.
//
// Use this method for communication with localhost. For example, with the inspector or debugging services.
// Otherwise in most cases DialNode should be used for communicating with nodes since it is secure.
func DialAddressEncrypted(ctx context.Context, address string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	options := append([]grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		})),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
	}, opts...)

	ctx, cf := context.WithTimeout(ctx, timeout)
	defer cf()

	conn, err = grpc.DialContext(ctx, address, options...)
	if err == context.Canceled {
		return nil, err
	}
	return conn, Error.Wrap(err)
}
