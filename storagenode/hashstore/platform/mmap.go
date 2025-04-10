// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package platform

import (
	"os"
	"syscall"
)

// PageSize is the size of a memory page on the system.
var PageSize = int64(syscall.Getpagesize())

// Mmap maps size bytes of the file into memory returning the byte slice and
// a function to close the mapping.
func Mmap(fh *os.File, size int) ([]byte, func() error, error) {
	return mmap(fh, size)
}

// Mremap remaps the data returned by mmap to be of the new size. The old data
// slice should not be used after a success.
func Mremap(data []byte, size int) ([]byte, error) {
	return mremap(data, size)
}

// AdviseRandom advises the kernel that the data will be accessed randomly.
func AdviseRandom(data []byte) {
	adviseRandom(data)
}

// AdviseSequential advises the kernel that the data will be accessed
// sequentially.
func AdviseSequential(data []byte) {
	adviseSequential(data)
}
