// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

const tcpFastOpen = 1025

func setTCPFastOpen(fd uintptr, _queue int) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpFastOpen, 1)
}

var tryInitFastOpenOnce sync.Once
var initFastOpenPossiblyEnabled bool

// tryInitFastOpen returns true if fastopen support is possibly enabled.
func tryInitFastOpen(log *zap.Logger) bool {
	tryInitFastOpenOnce.Do(func() {
		initFastOpenPossiblyEnabled = true
		output, err := exec.Command("sysctl", "-n", "net.inet.tcp.fastopen.server_enable").Output()
		if err != nil {
			log.Sugar().Infof("kernel support for tcp fast open unknown")
			initFastOpenPossiblyEnabled = true
			return
		}
		enabled, err := strconv.ParseBool(strings.TrimSpace(string(output)))
		if err != nil {
			log.Sugar().Infof("kernel support for tcp fast open unparsable")
			initFastOpenPossiblyEnabled = true
			return
		}
		if enabled {
			log.Sugar().Infof("kernel support for server-side tcp fast open enabled.")
		} else {
			log.Sugar().Infof("kernel support for server-side tcp fast open not enabled.")
			log.Sugar().Infof("enable with: sysctl net.inet.tcp.fastopen.server_enable=1")
			log.Sugar().Infof("enable on-boot by setting net.inet.tcp.fastopen.server_enable=1 in /etc/sysctl.conf")
		}
		initFastOpenPossiblyEnabled = enabled
	})
	return initFastOpenPossiblyEnabled
}
