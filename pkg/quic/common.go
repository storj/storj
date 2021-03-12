// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package quic

import (
	"net"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/sys/unix"
)

var (
	mon = monkit.Package()

	// Error is a pkg/quic error.
	Error = errs.Class("quic error")
)

// Experiments have shown that QUIC transfers on high-bandhwidth connections can
// be limited by the size of the UDP receive buffer. Therefore, we want to make
// sure the buffer size on the machine is able to take advantage of QUIC.
// See more detail: https://github.com/lucas-clemente/quic-go/wiki/UDP-Receive-Buffer-Size
const desiredReceiveBufferSize = (1 << 20) * 2 // 2MB

// HasSufficientUDPReceiveBufferSize checks whether an udp connection has enough
// udp receive buffer size that quic needs to be performant.
func HasSufficientUDPReceiveBufferSize(conn *net.UDPConn) bool {
	size, err := inspectReadBuffer(conn)
	return err == nil || size >= desiredReceiveBufferSize
}

func inspectReadBuffer(conn *net.UDPConn) (int, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return 0, errs.Wrap(err)
	}
	var size int
	var serr error
	if err := rawConn.Control(func(fd uintptr) {
		size, serr = unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF)
	}); err != nil {
		return 0, err
	}
	return size, serr
}
