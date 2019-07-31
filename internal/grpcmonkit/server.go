// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package grpcmonkit implements tracing grpc calls via monkit.
package grpcmonkit

import (
	"context"
	"reflect"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// UnaryServerInterceptor uses monkit to intercept server requests.
func UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	traceid, spanid := traceFromRequest(ctx)
	trace := monkit.NewTrace(traceid)

	service, endpoint := parseFullMethod(info.FullMethod)
	scope := monkit.ScopeNamed(service)
	fn := scope.FuncNamed(endpoint)
	defer fn.RemoteTrace(&ctx, spanid, trace)(&err)

	return handler(ctx, req)
}

// StreamServerInterceptor uses monkit to intercept server stream requests.
func StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	ctx := stream.Context()
	traceid, spanid := traceFromRequest(ctx)
	trace := monkit.NewTrace(traceid)

	service, endpoint := parseFullMethod(info.FullMethod)
	scope := monkit.ScopeNamed(service)
	fn := scope.FuncNamed(endpoint)
	defer fn.RemoteTrace(&ctx, spanid, trace)(&err)

	return handler(srv, &ServerStream{stream, scope, ctx})
}

func traceFromRequest(ctx context.Context) (traceid, spanid int64) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return monkit.NewId(), monkit.NewId()
	}

	traceIDs := md[traceIDKey]
	spanIDs := md[spanIDKey]
	if len(traceIDs) == 0 || len(spanIDs) == 0 {
		return monkit.NewId(), monkit.NewId()
	}

	var err error

	traceid, err = strconv.ParseInt(traceIDs[0], 10, 64)
	if err != nil {
		return monkit.NewId(), monkit.NewId()
	}

	spanid, err = strconv.ParseInt(spanIDs[0], 10, 64)
	if err != nil {
		return monkit.NewId(), monkit.NewId()
	}

	return traceid, spanid
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
