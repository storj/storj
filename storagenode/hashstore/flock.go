// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"os"
	"path/filepath"
)

func optimisticFlock(fh *os.File) error {
	if !flockSupported {
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
