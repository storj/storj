// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows && !linux && !darwin

package iopriority

import "syscall"

// SetLowIOPriority lowers the process I/O priority.
func SetLowIOPriority() error {
	// This might not necessarily affect the process I/O priority as POSIX does not
	// mandate the concept of I/O priority. However, it is a "best effort" to lower
	// the process I/O priority.
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, 9)
}
