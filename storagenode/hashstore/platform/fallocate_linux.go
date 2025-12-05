// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"syscall"
)

// Fallocate preallocates space for a file. It is a no-op on platforms that do
// not support it.
func Fallocate(fh *os.File, size int64) error {
	tmp, err := os.CreateTemp(filepath.Dir(fh.Name()), "fallocate-test-*.tmp")
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	if err := realFallocate(tmp, 1); err != nil {
		return nil
	}
	return realFallocate(fh, size)
}

func realFallocate(fh *os.File, size int64) error {
	return Error.Wrap(syscall.Fallocate(int(fh.Fd()), 0, 0, size))
}
