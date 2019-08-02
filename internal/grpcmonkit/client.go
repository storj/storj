// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcmonkit

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	zipkin "gopkg.in/spacemonkeygo/monkit-zipkin.v2"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// ClientDialOptions returns options for adding monkit tracing.
func ClientDialOptions() []grpc.DialOption {
	intercept := &ClientInterceptor{}
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(intercept.Unary),
		grpc.WithStreamInterceptor(intercept.Stream),
	}
}

// ClientInterceptor intercepts with traces
type ClientInterceptor struct{}

func appendSpanToOutgoingContext(ctx context.Context, span *monkit.Span) context.Context {
	zreq := zipkin.RequestFromSpan(span)

	md := make([]string, 0, 10)
	if zreq.TraceId != nil {
		md = append(md, traceIDKey, strconv.FormatInt(*zreq.TraceId, 16))
	}
	if zreq.SpanId != nil {
		md = append(md, spanIDKey, strconv.FormatInt(*zreq.SpanId, 16))
	}
	if zreq.ParentId != nil {
		md = append(md, parentIDKey, strconv.FormatInt(*zreq.ParentId, 16))
	}
	if zreq.Sampled != nil {
		md = append(md, traceIDKey, strconv.FormatBool(*zreq.Sampled))
	}
	if zreq.Flags != nil {
		md = append(md, flagsKey, strconv.FormatInt(*zreq.Flags, 16))
	}

	return metadata.AppendToOutgoingContext(ctx, md...)
}

// Unary intercepts RPC calls.
func (intercept *ClientInterceptor) Unary(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
	span := monkit.SpanFromCtx(ctx)
	span.Annotate("endpoint", method)
	ctx = appendSpanToOutgoingContext(ctx, span)
	err = invoker(ctx, method, req, reply, cc, opts...)
	if err != nil {
		span.Annotate("error", err.Error())
		return err
	}
	return nil
}

// Stream intercepts stream calls
func (intercept *ClientInterceptor) Stream(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (stream grpc.ClientStream, err error) {
	span := monkit.SpanFromCtx(ctx)
	span.Annotate("endpoint", method)
	ctx = appendSpanToOutgoingContext(ctx, span)
	stream, err = streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		span.Annotate("error", err.Error())
		return stream, err
	}
	return &ClientStream{ClientStream: stream, ctx: ctx}, nil
}

// ClientStream implements wrapping monkit server stream.
type ClientStream struct {
	grpc.ClientStream
	ctx context.Context
}

// Context returns the context for this stream.
func (stream *ClientStream) Context() context.Context { return stream.ctx }
