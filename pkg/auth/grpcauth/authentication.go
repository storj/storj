// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth

import (
	"context"
	"strings"

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
		APIKey := strings.Join(md["apikey"], "")
		if !ok || APIKey == "" {
			return handler(ctx, req)
		}
		return handler(auth.WithAPIKey(ctx, []byte(APIKey)), req)
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
