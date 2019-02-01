// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"storj.io/storj/pkg/auth"
)

// NewAPIKeyInterceptor creates instance of apikey interceptor
func NewAPIKeyInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
		err error) {

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return handler(ctx, req)
		}
		apikeys, ok := md["apikey"]
		if !ok || len(apikeys) == 0 {
			return handler(ctx, req)
		}

		return handler(auth.WithAPIKey(ctx, []byte(apikeys[0])), req)
	}
}

// NewAPIKeyInjector injects api key to grpc connection context
func NewAPIKeyInjector(APIKey string, callOpts ...grpc.CallOption) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		opts = append(opts, callOpts...)
		ctx = metadata.AppendToOutgoingContext(ctx, "apikey", APIKey)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
