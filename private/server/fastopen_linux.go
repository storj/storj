// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

const tcpFastOpen = 0x17
const tcpFastOpenSysctlPath = "/proc/sys/net/ipv4/tcp_fastopen"

func setTCPFastOpen(fd uintptr, queue int) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpFastOpen, queue)
}

var tryInitFastOpenOnce sync.Once

func tryInitFastOpen(log *zap.Logger) {
	tryInitFastOpenOnce.Do(func() {
		data, err := os.ReadFile(tcpFastOpenSysctlPath)
		if err != nil {
			log.Sugar().Infof("kernel support for tcp fast open unknown")
			return
		}
		fastOpenFlags, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			log.Sugar().Infof("kernel support for tcp fast open unparsable")
			return
		}
		if fastOpenFlags&0x2 != 0 {
			log.Sugar().Infof("existing kernel support for server-side tcp fast open detected")
			return
		}
		err = os.WriteFile(tcpFastOpenSysctlPath, []byte(fmt.Sprint(fastOpenFlags|0x2)), 0o644)
		if err != nil {
			log.Sugar().Infof("kernel support for server-side tcp fast open remains disabled.")

			// really, it's just the secondmost least significant bit that needs to
			// be flipped, but maybe this isn't the place to explain that. 0x3 will
			// enable standard fast open with standard cookies for both clients and
			// servers, so it's probably the right advice.
			log.Sugar().Infof("enable with: sysctl -w net.ipv4.tcp_fastopen=3")
			return
		}
		log.Sugar().Infof("kernel support for server-side tcp fast open enabled.")
	})
}
