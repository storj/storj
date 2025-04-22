// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux

package platform

import "os"

// MmapSupported is true if mmap is supported on the platform.
const MmapSupported = false

func mmap(fh *os.File, size int) ([]byte, error) {
	return nil, Error.New("not implemented")
}

func munmap(data []byte) error {
	return Error.New("not implemented")
}

func mremap(data []byte, size int) ([]byte, error) {
	return nil, Error.New("not implemented")
}

func mlock(data []byte) error {
	return Error.New("not implemented")
}

func adviseRandom(data []byte)     {}
func adviseSequential(data []byte) {}
