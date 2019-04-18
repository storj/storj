// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
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

// InvokeStreamTimeout enables timeouts for send/recv/close stream requests
type InvokeStreamTimeout struct {
	Timeout time.Duration
}

type clientStreamWrapper struct {
	timeout  time.Duration
	stream   grpc.ClientStream
	grpcConn *grpc.ClientConn
}

func (wrapper *clientStreamWrapper) Header() (metadata.MD, error) {
	return wrapper.stream.Header()
}

func (wrapper *clientStreamWrapper) Trailer() metadata.MD {
	return wrapper.stream.Trailer()
}

func (wrapper *clientStreamWrapper) Context() context.Context {
	return wrapper.stream.Context()
}

func (wrapper *clientStreamWrapper) CloseSend() error {
	return wrapper.addTimout(func() error {
		return wrapper.stream.CloseSend()
	})
}

func (wrapper *clientStreamWrapper) RecvMsg(m interface{}) error {
	return wrapper.addTimout(func() error {
		return wrapper.stream.RecvMsg(m)
	})
}

func (wrapper *clientStreamWrapper) SendMsg(m interface{}) error {
	return wrapper.addTimout(func() error {
		return wrapper.stream.SendMsg(m)
	})
}

func (wrapper *clientStreamWrapper) addTimout(f func() error) error {
	timoutTicker := time.NewTicker(wrapper.timeout)
	defer timoutTicker.Stop()
	errChannel := make(chan error)

	go func() {
		errChannel <- f()
	}()

	select {
	case <-timoutTicker.C:
		var errClose error
		if wrapper.grpcConn.GetState() != connectivity.Shutdown {
			errClose = wrapper.grpcConn.Close()
		}
		return errs.Combine(context.DeadlineExceeded, errClose)
	case err := <-errChannel:
		return err
	}
}

// Intercept adds a timeout to a stream requests
func (it InvokeStreamTimeout) Intercept(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	stream, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return stream, err
	}
	return &clientStreamWrapper{timeout: it.Timeout, stream: stream, grpcConn: cc}, nil
}
