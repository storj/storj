// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package grpcmonkit implements tracing grpc calls via monkit.
package grpcmonkit

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	zipkin "gopkg.in/spacemonkeygo/monkit-zipkin.v2"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	grpcmon = monkit.ScopeNamed("grpc")
)

// UnaryServerInterceptor uses monkit to intercept server requests.
func UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	trace, spanid := traceFromRequest(ctx)
	defer grpcmon.FuncNamed(info.FullMethod).RemoteTrace(&ctx, spanid, trace)(&err)
	span := monkit.SpanFromCtx(ctx)
	resp, err = handler(ctx, req)
	if err != nil {
		span.Annotate("error", err.Error())
		return resp, err
	}
	return resp, nil
}

// StreamServerInterceptor uses monkit to intercept server stream requests.
func StreamServerInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	ctx := stream.Context()
	trace, spanid := traceFromRequest(ctx)
	defer grpcmon.FuncNamed(info.FullMethod).RemoteTrace(&ctx, spanid, trace)(&err)
	span := monkit.SpanFromCtx(ctx)
	err = handler(srv, &ServerStream{ServerStream: stream, ctx: ctx})
	if err != nil {
		span.Annotate("error", err.Error())
		return err
	}
	return nil

}

func parseInt(a []string) *int64 {
	if len(a) != 1 {
		return nil
	}
	v, err := strconv.ParseInt(a[0], 16, 64)
	if err != nil {
		return nil
	}
	return &v
}

func traceFromRequest(ctx context.Context) (trace *monkit.Trace, spanid int64) {
	var zreq zipkin.Request

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		zreq.TraceId = parseInt(md[traceIDKey])
		zreq.SpanId = parseInt(md[spanIDKey])
		zreq.ParentId = parseInt(md[parentIDKey])
		zreq.Flags = parseInt(md[flagsKey])
		if sampleds := md[sampledKey]; len(sampleds) == 1 {
			if sampled, err := strconv.ParseBool(sampleds[0]); err == nil {
				zreq.Sampled = &sampled
			}
		}
	}

	return zreq.Trace()
}

// ServerStream implements wrapping monkit server stream.
type ServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the context for this stream.
func (stream *ServerStream) Context() context.Context { return stream.ctx }
