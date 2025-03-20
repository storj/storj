// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package platform

import "os"

// MMAP maps size bytes of the file into memory returning the byte slice and
// a function to close the mapping.
func MMAP(fh *os.File, size int) ([]byte, func() error, error) {
	return mmap(fh, size)
}
