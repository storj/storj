// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build linux

package server

import (
	"net"
	"runtime"

	"golang.org/x/sys/unix"
)

// defaultUserTimeout is the value we use for the TCP_USER_TIMEOUT setting.
const defaultUserTimeout = 60 * 1000 // 60s in ms

// wrapListener wraps the provided net.Listener in one that sets timeouts
// and monitors if the returned connections are closed or leaked.
func wrapListener(lis net.Listener) net.Listener {
	if lis, ok := lis.(*net.TCPListener); ok {
		return newUserTimeoutListener(lis)
	}
	return lis
}

// userTimeoutListener wraps a tcp listener so that it sets the TCP_USER_TIMEOUT
// value for each socket it returns.
type userTimeoutListener struct {
	lis *net.TCPListener
}

// newUserTimeoutListener wraps the tcp listener in a userTimeoutListener.
func newUserTimeoutListener(lis *net.TCPListener) *userTimeoutListener {
	return &userTimeoutListener{lis: lis}
}

// Accept waits for and returns the next connection to the listener.
func (lis *userTimeoutListener) Accept() (net.Conn, error) {
	conn, err := lis.lis.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// By default from Go, keep alive period + idle are ~15sec. The default
	// keep count is 8 according to some kernel docs. That means it should
	// fail after ~120 seconds. Unfortunately, keep alive only happens if
	// there is no send-q on the socket, and so a slow reader can still cause
	// hanging sockets forever. By setting user timeout, we will kill the
	// connection if any writes go unacknowledged for the amount of time.
	// This should close the keep alive hole.
	//
	// See https://blog.cloudflare.com/when-tcp-sockets-refuse-to-die/

	rawConn, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}
	controlErr := rawConn.Control(func(fd uintptr) {
		err = unix.SetsockoptInt(int(fd), unix.SOL_TCP, unix.TCP_USER_TIMEOUT, defaultUserTimeout)
	})
	if controlErr != nil {
		return nil, controlErr
	}
	if err != nil {
		return nil, err
	}

	return newCloseTrackingConn(conn), nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (lis *userTimeoutListener) Close() error {
	return lis.lis.Close()
}

// Addr returns the listener's network address.
func (lis *userTimeoutListener) Addr() net.Addr {
	return lis.lis.Addr()
}

// closeTrackingConn wraps a net.Conn and keeps track of if it was closed
// or if it was leaked (and closes it if it was leaked.)
type closeTrackingConn struct {
	net.Conn
}

// newCloseTrackingConn wraps the conn in a closeTrackingConn. It sets a
// finalizer on the returned value to close the conn and monitor that it
// was leaked.
func newCloseTrackingConn(conn net.Conn) *closeTrackingConn {
	tracked := &closeTrackingConn{Conn: conn}
	runtime.SetFinalizer(tracked, (*closeTrackingConn).finalize)
	return tracked
}

// Close clears the finalizer and closes the connection.
func (c *closeTrackingConn) Close() error {
	runtime.SetFinalizer(c, nil)
	mon.Event("connection_closed")
	return c.Conn.Close()
}

// finalize monitors that a connection was leaked and closes the connection.
func (c *closeTrackingConn) finalize() {
	mon.Event("connection_leaked")
	_ = c.Conn.Close()
}
