// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcutils

import (
	"context"

	"google.golang.org/grpc"
)

// CombineStreamServerInterceptors takes one or more grpc.StreamServerInterceptor and stacks them together into one.
// Call order is left-to-right. The left-most interceptor is called first, with the right-most interceptor having the longest callstack.
func CombineStreamServerInterceptors(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	switch len(interceptors) {
	case 0:
		return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	case 1:
		return interceptors[0]
	}

	remaining := CombineStreamServerInterceptors(interceptors[1:]...)
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return interceptors[0](srv, ss, info, func(srv interface{}, ss grpc.ServerStream) error {
			return remaining(srv, ss, info, handler)
		})
	}
}

// CombineUnaryServerInterceptors takes one or more grpc.UnaryServerInterceptor and stacks them together into one.
// Call order is left-to-right. The left-most interceptor is called first, with the right-most interceptor having the longest callstack.
func CombineUnaryServerInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	switch len(interceptors) {
	case 0:
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	case 1:
		return interceptors[0]
	}

	remaining := CombineUnaryServerInterceptors(interceptors[1:]...)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return interceptors[0](ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return remaining(ctx, req, info, handler)
		})
	}
}

// CombineStreamClientInterceptors takes one or more grpc.StreamClientInterceptor and stacks them together into one.
// Call order is left-to-right. The left-most interceptor is called first, with the right-most interceptor having the longest callstack.
func CombineStreamClientInterceptors(interceptors ...grpc.StreamClientInterceptor) grpc.StreamClientInterceptor {
	switch len(interceptors) {
	case 0:
		return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(ctx, desc, cc, method, opts...)
		}
	case 1:
		return interceptors[0]
	}

	remaining := CombineStreamClientInterceptors(interceptors[1:]...)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return interceptors[0](ctx, desc, cc, method, func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return remaining(ctx, desc, cc, method, streamer, opts...)
		}, opts...)
	}
}

// CombineUnaryClientInterceptors takes one or more grpc.UnaryClientInterceptor and stacks them together into one.
// Call order is left-to-right. The left-most interceptor is called first, with the right-most interceptor having the longest callstack.
func CombineUnaryClientInterceptors(interceptors ...grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	switch len(interceptors) {
	case 0:
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	case 1:
		return interceptors[0]
	}

	remaining := CombineUnaryClientInterceptors(interceptors[1:]...)
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return interceptors[0](ctx, method, req, reply, cc, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			return remaining(ctx, method, req, reply, cc, invoker, opts...)
		}, opts...)
	}
}
