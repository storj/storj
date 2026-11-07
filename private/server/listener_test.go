// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTCPUserTimeoutListener_DropsConnOnSetupError(t *testing.T) {
	calls := 0
	orig := setUserTimeout
	t.Cleanup(func() { setUserTimeout = orig })
	setUserTimeout = func(conn *net.TCPConn, timeout time.Duration) error {
		calls++
		if calls == 1 {
			return syscall.EOPNOTSUPP
		}
		return nil
	}

	tcpLis, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	lis := newTCPUserTimeoutListener(tcpLis)
	defer func() { _ = lis.Close() }()

	// the first connection fails setup and must be dropped, the second one
	// must be returned by Accept.
	var dialer net.Dialer
	first, err := dialer.DialContext(t.Context(), "tcp", lis.Addr().String())
	require.NoError(t, err)
	defer func() { _ = first.Close() }()

	second, err := dialer.DialContext(t.Context(), "tcp", lis.Addr().String())
	require.NoError(t, err)
	defer func() { _ = second.Close() }()

	conn, err := lis.Accept()
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.Equal(t, 2, calls)
	require.Equal(t, second.LocalAddr().String(), conn.RemoteAddr().String())

	// the dropped connection is closed by the listener.
	require.NoError(t, first.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = first.Read(make([]byte, 1))
	require.Error(t, err)
}
