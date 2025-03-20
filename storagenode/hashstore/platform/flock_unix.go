// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build unix

package platform

import (
	"os"
	"syscall"
)

// FlockSupported is a constant indicating if flock is supported on the platform.
const FlockSupported = true

func flock(fh *os.File) error {
	return Error.Wrap(syscall.Flock(
		int(fh.Fd()),
		syscall.LOCK_EX|syscall.LOCK_NB,
	))
}
