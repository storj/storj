// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
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

// InvokeStreamTimeout enables timeouts for send/recv/close stream requests
type InvokeStreamTimeout struct {
	Timeout time.Duration
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
	// deadline needs to be set before each write operation
	err = conn.SetWriteDeadline(time.Now().Add(conn.timeout))
	if err != nil {
		return 0, err
	}
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
