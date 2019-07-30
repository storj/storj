// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcmonkit

import (
	"context"
	"reflect"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// ClientDialOptions returns options for adding monkit tracing.
func ClientDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(NewUnaryClientInterceptor()),
		grpc.WithStreamInterceptor(NewStreamClientInterceptor()),
	}
}

// NewUnaryClientInterceptor creates an monkit client interceptor.
func NewUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	trace := monkit.NewTrace(monkit.NewId())

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		spanid := monkit.NewId()
		ctx = metadata.AppendToOutgoingContext(ctx, traceIDKey, strconv.FormatInt(trace.Id(), 10))
		ctx = metadata.AppendToOutgoingContext(ctx, spanIDKey, strconv.FormatInt(spanid, 10))

		service, endpoint := parseFullMethod(method)
		scope := monkit.ScopeNamed(service)
		fn := scope.FuncNamed(endpoint)
		defer fn.RemoteTrace(&ctx, spanid, trace)(&err)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// NewStreamClientInterceptor creates an monkit client stream interceptor.
func NewStreamClientInterceptor() grpc.StreamClientInterceptor {
	trace := monkit.NewTrace(monkit.NewId())

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (stream grpc.ClientStream, err error) {
		spanid := monkit.NewId()
		ctx = metadata.AppendToOutgoingContext(ctx, traceIDKey, strconv.FormatInt(trace.Id(), 10))
		ctx = metadata.AppendToOutgoingContext(ctx, spanIDKey, strconv.FormatInt(spanid, 10))

		service, endpoint := parseFullMethod(method)
		scope := monkit.ScopeNamed(service)
		fn := scope.FuncNamed(endpoint)
		defer fn.RemoteTrace(&ctx, spanid, trace)(&err)

		stream, err = streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return stream, err
		}
		return &ClientStream{stream, scope, ctx}, err
	}
}

// ClientStream implements wrapping monkit server stream.
type ClientStream struct {
	grpc.ClientStream
	scope *monkit.Scope
	ctx   context.Context
}

// Context returns the context for this stream.
func (stream *ClientStream) Context() context.Context { return stream.ctx }

// SendMsg sends a message.
func (stream *ClientStream) SendMsg(m interface{}) (err error) {
	msgtype := reflect.TypeOf(m).String()
	ctx := stream.ctx
	defer stream.scope.TaskNamed(msgtype)(&ctx)(&err)
	return stream.ClientStream.SendMsg(m)
}

// RecvMsg blocks until it receives a message into m or the stream is done.
func (stream *ClientStream) RecvMsg(m interface{}) (err error) {
	msgtype := reflect.TypeOf(m).String()
	ctx := stream.ctx
	defer stream.scope.TaskNamed(msgtype)(&ctx)(&err)
	return stream.ClientStream.RecvMsg(m)
}
