// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package hashstore

import "os"

func createFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
}
