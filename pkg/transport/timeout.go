// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
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
	timeout time.Duration
	stream  grpc.ClientStream
	cancel  func()
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
	return wrapper.withTimeout(func() error {
		return wrapper.stream.CloseSend()
	})
}

func (wrapper *clientStreamWrapper) SendMsg(m interface{}) error {
	return wrapper.withTimeout(func() error {
		return wrapper.stream.SendMsg(m)
	})
}

func (wrapper *clientStreamWrapper) RecvMsg(m interface{}) error {
	return wrapper.withTimeout(func() error {
		return wrapper.stream.RecvMsg(m)
	})
}

func (wrapper *clientStreamWrapper) withTimeout(f func() error) error {
	timoutTicker := time.NewTicker(wrapper.timeout)
	defer timoutTicker.Stop()
	doneCh := make(chan struct{})
	defer close(doneCh)

	go func() {
		select {
		case <-timoutTicker.C:
			wrapper.cancel()
		case <-wrapper.Context().Done():
		case <-doneCh:
		}
	}()

	return f()
}

// Intercept adds a timeout to a stream requests
func (it InvokeStreamTimeout) Intercept(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (_ grpc.ClientStream, err error) {
	wrapper := &clientStreamWrapper{timeout: it.Timeout}
	ctx, wrapper.cancel = context.WithCancel(ctx)

	wrapper.stream, err = streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return wrapper.stream, err
	}
	return wrapper, nil
}

type timeoutConn struct {
	conn    net.Conn
	timeout time.Duration
}

func (conn *timeoutConn) Read(b []byte) (n int, err error) {
	// deadline needs to be set before each read operation
	err = conn.SetReadDeadline(time.Now().Add(conn.timeout))
	if err != nil {
		return 0, err
	}
	return conn.conn.Read(b)
}

func (conn *timeoutConn) Write(b []byte) (n int, err error) {
	return conn.conn.Write(b)
}

func (conn *timeoutConn) Close() error {
	return conn.conn.Close()
}

func (conn *timeoutConn) LocalAddr() net.Addr {
	return conn.conn.LocalAddr()
}

func (conn *timeoutConn) RemoteAddr() net.Addr {
	return conn.conn.RemoteAddr()
}

func (conn *timeoutConn) SetDeadline(t time.Time) error {
	return conn.conn.SetDeadline(t)
}

func (conn *timeoutConn) SetReadDeadline(t time.Time) error {
	return conn.conn.SetReadDeadline(t)
}

func (conn *timeoutConn) SetWriteDeadline(t time.Time) error {
	return conn.conn.SetWriteDeadline(t)
}
