// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux

package platform

import (
	"os"

	"golang.org/x/sys/unix"
)

// MmapSupported is true if mmap is supported on the platform.
const MmapSupported = true

func mmap(fh *os.File, size int) ([]byte, func() error, error) {
	if size < 0 || uint64(size) > uint64(^uintptr(0)) {
		return nil, nil, Error.New("size out of range")
	}

	data, err := unix.Mmap(
		int(fh.Fd()),
		0,
		size,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return data, func() error { return unix.Munmap(data) }, nil
}

func mremap(data []byte, size int) ([]byte, error) {
	return unix.Mremap(data, size, unix.MREMAP_MAYMOVE)
}

func adviseRandom(data []byte) {
	_ = unix.Madvise(data, unix.MADV_RANDOM)
}

func adviseSequential(data []byte) {
	_ = unix.Madvise(data, unix.MADV_SEQUENTIAL)
}
