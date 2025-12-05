// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"net"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/netutil"
	"storj.io/common/rpc"
	"storj.io/common/rpc/quic"
)

// defaultUserTimeout is the value we use for the TCP_USER_TIMEOUT setting.
const defaultUserTimeout = 60 * time.Second

// wrapListener wraps the provided net.Listener in one that sets timeouts
// and monitors if the returned connections are closed or leaked.
func wrapListener(lis net.Listener) net.Listener {
	if lis, ok := lis.(*net.TCPListener); ok {
		return newTCPUserTimeoutListener(lis)
	}
	if lis, ok := lis.(*quic.Listener); ok {
		return newQUICTrackedListener(lis)
	}
	return lis
}

// tcpUserTimeoutListener wraps a tcp listener so that it sets the TCP_USER_TIMEOUT
// value for each socket it returns.
type tcpUserTimeoutListener struct {
	lis *net.TCPListener
}

// newTCPUserTimeoutListener wraps the tcp listener in a userTimeoutListener.
func newTCPUserTimeoutListener(lis *net.TCPListener) *tcpUserTimeoutListener {
	return &tcpUserTimeoutListener{lis: lis}
}

// Accept waits for and returns the next connection to the listener.
func (lis *tcpUserTimeoutListener) Accept() (net.Conn, error) {
	conn, err := lis.lis.AcceptTCP()
	if err != nil {
		return nil, err
	}
	mon.Event("incoming_connection", monkit.NewSeriesTag("kind", "tcp"))

	if err := netutil.SetUserTimeout(conn, defaultUserTimeout); err != nil {
		return nil, errs.Combine(err, conn.Close())
	}
	return netutil.TrackClose(conn), nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (lis *tcpUserTimeoutListener) Close() error {
	return lis.lis.Close()
}

// Addr returns the listener's network address.
func (lis *tcpUserTimeoutListener) Addr() net.Addr {
	return lis.lis.Addr()
}

type quicTrackedListener struct {
	lis *quic.Listener
}

func newQUICTrackedListener(lis *quic.Listener) *quicTrackedListener {
	return &quicTrackedListener{lis: lis}
}

func (lis *quicTrackedListener) Accept() (net.Conn, error) {
	conn, err := lis.lis.Accept()
	if err != nil {
		return nil, err
	}
	mon.Event("incoming_connection", monkit.NewSeriesTag("kind", "quic"))

	connectorConn, ok := conn.(rpc.ConnectorConn)
	if !ok {
		return nil, Error.New("quic connection doesn't implement required methods")
	}

	return quic.TrackClose(connectorConn), nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (lis *quicTrackedListener) Close() error {
	return lis.lis.Close()
}

// Addr returns the listener's network address.
func (lis *quicTrackedListener) Addr() net.Addr {
	return lis.lis.Addr()
}
