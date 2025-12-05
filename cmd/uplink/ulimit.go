// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux || darwin || freebsd

package main

import "syscall"

// raise RLIMIT_NOFILE softlimit to hardlimit.
func raiseUlimits() {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return
	}
	if rLimit.Cur < rLimit.Max {
		rLimit.Cur = rLimit.Max
		_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	}
}
