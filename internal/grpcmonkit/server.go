// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package grpcmonkit implements tracing grpc calls via monkit.
package grpcmonkit

import (
	"context"
	"reflect"

	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// NewUnaryServerInterceptor creates an monkit server interceptor.
func NewUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		service, endpoint := parseFullMethod(info.FullMethod)
		scope := monkit.ScopeNamed(service)
		defer scope.TaskNamed(endpoint)(&ctx, req)(&err)

		return handler(ctx, req)
	}
}

// NewStreamServerInterceptor creates an monkit server stream interceptor.
func NewStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		service, endpoint := parseFullMethod(info.FullMethod)
		scope := monkit.ScopeNamed(service)
		ctx := stream.Context()
		defer scope.TaskNamed(endpoint)(&ctx)(&err)

		return handler(srv, &ServerStream{stream, scope, ctx})
	}
}

// ServerStream implements wrapping monkit server stream.
type ServerStream struct {
	grpc.ServerStream
	scope *monkit.Scope
	ctx   context.Context
}

// Context returns the context for this stream.
func (stream *ServerStream) Context() context.Context { return stream.ctx }

// SendMsg sends a message.
func (stream *ServerStream) SendMsg(m interface{}) (err error) {
	msgtype := reflect.TypeOf(m).String()
	ctx := stream.ctx
	defer stream.scope.TaskNamed(msgtype)(&ctx)(&err)
	return stream.ServerStream.SendMsg(m)
}

// RecvMsg blocks until it receives a message into m or the stream is done.
func (stream *ServerStream) RecvMsg(m interface{}) (err error) {
	msgtype := reflect.TypeOf(m).String()
	ctx := stream.ctx
	defer stream.scope.TaskNamed(msgtype)(&ctx)(&err)
	return stream.ServerStream.RecvMsg(m)
}
