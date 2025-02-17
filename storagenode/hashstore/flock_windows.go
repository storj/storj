// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build windows

package hashstore

import (
	"os"

	"golang.org/x/sys/windows"
)

const flockSupported = true

func flock(fh *os.File) error {
	return Error.Wrap(windows.LockFileEx(
		windows.Handle(fh.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,          // reserved
		0,          // bytes low
		^uint32(0), // bytes high
		new(windows.Overlapped),
	))
}
