// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth

import (
	"context"
	"strings"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/grpcutils"
)

// NewAPIKeyInterceptor creates instance of apikey interceptor
func NewAPIKeyInterceptor() grpcutils.ServerInterceptor {
	authfunc := grpc_auth.AuthFunc(
		func(ctx context.Context) (context.Context, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			APIKey := strings.Join(md["apikey"], "")
			if !ok || APIKey == "" {
				return ctx, nil
			}
			return auth.WithAPIKey(ctx, []byte(APIKey)), nil
		})
	return grpcutils.ServerInterceptor{
		Unary:  grpc_auth.UnaryServerInterceptor(authfunc),
		Stream: grpc_auth.StreamServerInterceptor(authfunc),
	}
}

// NewAPIKeyInjector injects api key to grpc connection context
func NewAPIKeyInjector(APIKey string) grpcutils.ClientInterceptor {
	return grpcutils.ClientInterceptor{
		Unary: func(ctx context.Context, method string, req, reply interface{},
			cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (
			err error) {
			return invoker(metadata.AppendToOutgoingContext(ctx, "apikey", APIKey),
				method, req, reply, cc, opts...)
		},
		Stream: func(ctx context.Context, desc *grpc.StreamDesc,
			cc *grpc.ClientConn, method string, streamer grpc.Streamer,
			opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(metadata.AppendToOutgoingContext(ctx, "apikey", APIKey),
				desc, cc, method, opts...)
		},
	}
}
