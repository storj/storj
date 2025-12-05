// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package platform

import "os"

// CreateFile creates a file in read/write mode that errors if it already exists.
func CreateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
}

// Rename atomically renames a file, replacing the destination if it exists.
func Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
