// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package platform

import (
	"os"
	"syscall"
)

// PageSize is the size of a memory page on the system.
var PageSize = int64(syscall.Getpagesize())

// Mmap maps size bytes of the file into memory returning the byte slice.
func Mmap(fh *os.File, size int) ([]byte, error) {
	return mmap(fh, size)
}

// Munmap unmaps the data returned by mmap. The data slice should not be used
// after a success.
func Munmap(data []byte) error {
	return munmap(data)
}

// Mremap remaps the data returned by mmap to be of the new size. The old data
// slice should not be used after a success.
func Mremap(data []byte, size int) ([]byte, error) {
	return mremap(data, size)
}

// Mlock locks the data pages into memory so that they won't be paged out.
func Mlock(data []byte) error {
	return mlock(data)
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
