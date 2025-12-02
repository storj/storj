// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package event

import (
	"context"
	"strings"

	"storj.io/drpc"
)

// Handler is a drpc handler wrapper that creates events for RPC calls.
type Handler struct {
	delegate drpc.Handler
}

// WrapHandler wraps a drpc handler to enable event tracking for RPC calls.
func WrapHandler(delegate drpc.Handler) Handler {
	return Handler{delegate: delegate}
}

// HandleRPC handles an RPC call with event tracking and context propagation.
func (h Handler) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	endpoint, method, ok := strings.Cut(strings.Trim(rpc, "/"), "/")
	if !ok {
		return h.delegate.HandleRPC(stream, rpc)
	}
	ctx := stream.Context()
	nctx, report := StartEvent(ctx, strings.ToLower(endpoint))
	defer report()
	Annotate(nctx, "method", method)

	wrappedStream := &streamWrapper{
		Stream: stream,
		ctx:    nctx,
	}
	return h.delegate.HandleRPC(wrappedStream, rpc)
}

var _ drpc.Handler = Handler{}

type streamWrapper struct {
	drpc.Stream
	ctx context.Context
}

func (s *streamWrapper) Context() context.Context { return s.ctx }
