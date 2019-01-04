// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcauth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/grpcutils"
)

// processAuthMetadata loads the gRPC metadata, looks for gRPC-specific
// auth information, and adjusts the context to have the auth information
// discoverable through pkg/auth. ok is true if auth information was added,
// false if not, but in either case, a valid context is returned.
func processAuthMetadata(ctx context.Context) (_ context.Context, ok bool) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if key := strings.Join(md["apikey"], ""); key != "" {
			return auth.WithAPIKey(ctx, []byte(key)), true
		}
	}
	return ctx, false
}

// NewAPIKeyInterceptor creates instance of apikey interceptor
func NewAPIKeyInterceptor() grpcutils.ServerInterceptor {
	return grpcutils.ServerInterceptor{
		Unary: func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			ctx, _ = processAuthMetadata(ctx)
			return handler(ctx, req)
		},
		Stream: func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			if ctx, ok := processAuthMetadata(stream.Context()); ok {
				return handler(srv, &contextStream{ServerStream: stream, ctx: ctx})
			}
			return handler(srv, stream)
		},
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

// contextStream allows for tying a new context to an existing ServerStream
type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (cs *contextStream) Context() context.Context {
	return cs.ctx
}
