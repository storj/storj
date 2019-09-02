// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
)

// WithRequestTimeout defines request timeout (read/write) for grpc call
func WithRequestTimeout(timeout time.Duration) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		return &timeoutConn{conn: conn, timeout: timeout}, nil
	})
}

type timeoutConn struct {
	conn    net.Conn
	timeout time.Duration
}

func (tc *timeoutConn) Read(b []byte) (n int, err error) {
	// deadline needs to be set before each read operation
	err = tc.SetReadDeadline(time.Now().Add(tc.timeout))
	if err != nil {
		return 0, err
	}
	return tc.conn.Read(b)
}

func (tc *timeoutConn) Write(b []byte) (n int, err error) {
	// deadline needs to be set before each write operation
	err = tc.SetWriteDeadline(time.Now().Add(tc.timeout))
	if err != nil {
		return 0, err
	}
	return tc.conn.Write(b)
}

func (tc *timeoutConn) Close() error {
	return tc.conn.Close()
}

func (tc *timeoutConn) LocalAddr() net.Addr {
	return tc.conn.LocalAddr()
}

func (tc *timeoutConn) RemoteAddr() net.Addr {
	return tc.conn.RemoteAddr()
}

func (tc *timeoutConn) SetDeadline(t time.Time) error {
	return tc.conn.SetDeadline(t)
}

func (tc *timeoutConn) SetReadDeadline(t time.Time) error {
	return tc.conn.SetReadDeadline(t)
}

func (tc *timeoutConn) SetWriteDeadline(t time.Time) error {
	return tc.conn.SetWriteDeadline(t)
}
