// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcmonkit

import (
	"context"

	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// NewUnaryClientInterceptor creates an monkit client interceptor.
func NewUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		service, endpoint := parseFullMethod(method)
		scope := monkit.ScopeNamed(service)
		defer scope.TaskNamed(endpoint)(&ctx, req)(&err)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// NewStreamClientInterceptor creates an monkit client stream interceptor.
func NewStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (stream grpc.ClientStream, err error) {
		service, endpoint := parseFullMethod(method)
		scope := monkit.ScopeNamed(service)
		defer scope.TaskNamed(endpoint)(&ctx)(&err)

		stream, err = streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return stream, err
		}
		return &ClientStream{stream, ctx}, err
	}
}

// ClientStream implements wrapping monkit server stream.
type ClientStream struct {
	grpc.ClientStream
	ctx context.Context
}

// Context returns the context for this stream.
func (stream *ClientStream) Context() context.Context { return stream.ctx }
