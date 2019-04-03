// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// InvokeTimeout enables timeouts for requests that take too long
type InvokeTimeout struct {
	Timeout time.Duration
}

// Intercept adds a context timeout to a method call
func (it InvokeTimeout) Intercept(ctx context.Context, method string, req interface{}, reply interface{},
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	timedCtx, cancel := context.WithTimeout(ctx, it.Timeout)
	defer cancel()
	return invoker(timedCtx, method, req, reply, cc, opts...)
}
