// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"syscall"

	"go.uber.org/zap"
)

const tcpFastOpen = 0x17

func setTCPFastOpen(fd uintptr, queue int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_TCP, tcpFastOpen, queue)
}

func tryInitFastOpen(*zap.Logger) {}
