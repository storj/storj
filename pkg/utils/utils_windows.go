// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"os"

	"github.com/zeebo/errs"
)

// IsWritable determines if a directory is writeable
func IsWritable(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if !info.IsDir() {
		return false, errs.New("Path %s is not a directory", path)
	}

	// Check if the user bit is enabled in file permission
	if info.Mode().Perm()&(1<<(uint(7))) == 0 {
		return false, errs.New("Write permission bit is not set on this file for user")
	}

	return true, nil
}
