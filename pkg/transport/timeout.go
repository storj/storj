// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"net"
	"time"
)

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
