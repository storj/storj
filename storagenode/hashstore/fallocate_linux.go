// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux

package hashstore

import (
	"os"
	"syscall"
)

func fallocate(fh *os.File, size int64) error {
	return Error.Wrap(syscall.Fallocate(int(fh.Fd()), 0, 0, size))
}
