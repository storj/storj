// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package platform

import (
	"os"
	"path/filepath"
)

// Flock attempts to flock the file if flock is supported on the platform.
func Flock(fh *os.File) error {
	if !FlockSupported {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(fh.Name()), "flock-test-*.tmp")
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	if err := flock(tmp); err != nil {
		return nil
	}
	return flock(fh)
}
