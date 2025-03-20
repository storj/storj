// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows && !unix

package platform

import "os"

func mmap(fh *os.File, size int) ([]byte, func() error, error) {
	return nil, nil, Error.New("not implemented")
}
