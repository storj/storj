// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"

	"github.com/zeebo/errs"
)

// swapBinaries swaps the binaries with best effort.
//
// Note, this approach does not work that well for Windows, because it does not usually allow
// for renaming running processes.
func swapBinaries(ctx context.Context, binaryLocation, newVersionPath, backupPath string) error {
	if err := os.Rename(binaryLocation, backupPath); err != nil {
		return errs.Wrap(err)
	}

	if err := os.Rename(newVersionPath, binaryLocation); err != nil {
		return errs.Combine(err, os.Rename(backupPath, binaryLocation), os.Remove(newVersionPath))
	}

	return nil
}
