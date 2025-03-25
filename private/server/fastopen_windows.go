// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

const tcpFastOpen = 15 // Corresponds to TCP_FASTOPEN from MS SDK

func setTCPFastOpen(fd uintptr, queue int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_TCP, tcpFastOpen, 1)
}

var tryInitFastOpenOnce sync.Once
var initFastOpenPossiblyEnabled bool

// tryInitFastOpen returns true if fastopen support is possibly enabled.
func tryInitFastOpen(*zap.Logger) bool {
	tryInitFastOpenOnce.Do(func() {
		// TCP-FASTOPEN is supported as of Windows 10 build 1607, but is
		// enabled per socket. If the socket option isn't supported then the
		// call to opt-in will fail. So as long as we can set up a listening
		// socket with the right socket option set, we should be good.
		if listener, err := (&net.ListenConfig{
			Control: func(network, addr string, c syscall.RawConn) error {
				var sockOptErr error
				if controlErr := c.Control(func(fd uintptr) {
					sockOptErr = setTCPFastOpen(fd, 0) // queue is unused
				}); controlErr != nil {
					return controlErr
				}
				return sockOptErr
			},
		}).Listen(context.Background(), "tcp", "127.0.0.1:0"); err == nil {
			_ = listener.Close()
			initFastOpenPossiblyEnabled = true
		}
	})
	return initFastOpenPossiblyEnabled
}
