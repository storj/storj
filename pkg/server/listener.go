// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"net"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/netutil"
)

// defaultUserTimeout is the value we use for the TCP_USER_TIMEOUT setting.
const defaultUserTimeout = 60 * time.Second

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

	if err := netutil.SetUserTimeout(conn, defaultUserTimeout); err != nil {
		return nil, errs.Combine(err, conn.Close())
	}
	return netutil.TrackClose(conn), nil
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
