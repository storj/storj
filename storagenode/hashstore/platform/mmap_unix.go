// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build unix

package platform

import (
	"os"
	"syscall"
)

func mmap(fh *os.File, size int) ([]byte, func() error, error) {
	if size < 0 || uint64(size) > uint64(^uintptr(0)) {
		return nil, nil, Error.New("size out of range")
	}

	data, err := syscall.Mmap(
		int(fh.Fd()),
		0,
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	// attempt to tell the kernel this is going to be random access.
	_ = syscall.Madvise(data, syscall.MADV_RANDOM)

	return data, func() error { return syscall.Munmap(data) }, nil
}
